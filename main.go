package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/gocolly/colly/v2"
	"golang.org/x/sync/errgroup"
)

type Word struct {
	word        string
	definitions []string
	url         string
}

func main() {
	// use run function to allow the use of os.Exit while allowing defer blocks to run.
	exitCode := run()

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func run() int {
	err := SetupDefaultLogger()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 12
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	recipients, err := getRecipients()
	if err != nil {
		slog.Error("failed to process recipients", "error", err)
		return 10
	}

	botToken := os.Getenv("WOD_RAE_BOT_TOKEN")
	if botToken == "" {
		slog.Error("WOD_RAE_BOT_TOKEN is not set or empty")
		return 11
	}

	wordOfTheDay, ok := scrapeRAE(ctx, "https://dle.rae.es/")
	if !ok {
		return 1
	}

	if wordOfTheDay.word == "" {
		slog.Error("failed to get a word")
		return 1
	}

	if len(wordOfTheDay.definitions) == 0 {
		slog.Error("failed to get definitions", "word", wordOfTheDay.word)
		return 2
	}

	err = SendWordDefinition(ctx, botToken, recipients, wordOfTheDay)
	if err != nil {
		slog.Error("failed to send telegram messages", "error", err)
		return 20
	}

	return 0
}

// scrapeRAE extracts the word of the day from the RAE.
func scrapeRAE(ctx context.Context, rootURL string) (wordOfTheDay Word, ok bool) {
	frontCollector := colly.NewCollector(
		colly.StdlibContext(ctx),
	)
	wordCollector := frontCollector.Clone()
	ok = true

	frontCollector.OnHTML("a[href].c-word-day__link", func(h *colly.HTMLElement) {
		url := h.Request.AbsoluteURL(h.Attr("href"))
		slog.Debug("found word URL", "url", url)
		wordOfTheDay.url = url
		wordCollector.Visit(url)
	})

	frontCollector.OnHTML("span.c-word-day__word", func(h *colly.HTMLElement) {
		wordOfTheDay.word = h.Text
		slog.Info("found word of the day", "word", h.Text)
	})

	frontCollector.OnError(func(r *colly.Response, err error) {
		slog.Error("Error scraping front page", "request-url", r.Request.URL, "Response", r, "error", err)
	})

	wordCollector.OnHTML("div.c-definitions__item", func(h *colly.HTMLElement) {
		wordOfTheDay.definitions = append(wordOfTheDay.definitions, h.Text)
		slog.Debug("found word definition", "definition", h.Text)
	})

	frontCollector.OnError(func(r *colly.Response, err error) {
		slog.Error("Error scraping entry page", "request-url", r.Request.URL, "Response", r, "error", err)
		ok = false
	})

	frontCollector.Visit(rootURL)

	return
}

// getRecipients extracts the Telegram user IDs of the recipients from the envvar.
func getRecipients() ([]int64, error) {
	recipientsEnv := os.Getenv("WOD_RAE_RECIPIENTS")
	if recipientsEnv == "" {
		return nil, errors.New("WOD_RAE_RECIPIENTS is empty or not set")
	}

	recipientsStr := strings.Split(recipientsEnv, " ")
	recipients := make([]int64, 0, len(recipientsStr))

	for _, rec := range recipientsStr {
		i, err := strconv.ParseInt(rec, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error processing recipient ID %s: %w", rec, err)
		}
		recipients = append(recipients, i)
	}

	return recipients, nil
}

type TelegramResponse struct {
	Ok          bool   `json:"ok"`
	Result      any    `json:"result"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
}

// SendWordDefinition sends a telegram message to each recipientID with the word definition.
func SendWordDefinition(ctx context.Context, botToken string, recipientIDs []int64, wordData Word) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	// Sanitize URL for MarkdownV2
	escapedURL := escapeMarkdownV2(wordData.url)
	escapedWord := escapeMarkdownV2(wordData.word)

	message := fmt.Sprintf("*Palabra del día*:\n[*%s*](%s)\n", escapedWord, escapedURL)

	for _, def := range wordData.definitions {
		escapedDef := escapeMarkdownV2(def)
		message += fmt.Sprintf("%s\n", escapedDef)
	}

	slog.DebugContext(ctx, "created word definition message", "message", message)

	g, errCTX := errgroup.WithContext(ctx)
	// set a limit to avoid being rate-limited
	g.SetLimit(10)

	for _, recipientID := range recipientIDs {
		g.Go(func() error {
			params := url.Values{}
			params.Set("chat_id", fmt.Sprintf("%d", recipientID))
			params.Set("text", message)
			params.Set("parse_mode", "MarkdownV2")
			params.Set("disable_web_page_preview", "true")

			slog.DebugContext(errCTX, "sending telegram message", "recipientID", recipientID, "params", params)
			req, err := http.NewRequestWithContext(errCTX, "POST", apiURL, strings.NewReader(params.Encode()))
			if err != nil {
				return fmt.Errorf("error creating request for recipient %d: %w", recipientID, err)
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				select {
				case <-errCTX.Done():
					slog.DebugContext(errCTX, "sender routine cancelled by context", "recipientID", recipientID)
					return errCTX.Err() // Return context error if it was cancelled during the request
				default:
					return fmt.Errorf("error sending request to recipient %d: %w", recipientID, err)
				}
			}
			defer resp.Body.Close()

			var response TelegramResponse
			err = json.NewDecoder(resp.Body).Decode(&response)
			if err != nil {
				return fmt.Errorf("error decoding response from recipient %d: %w", recipientID, err)
			}

			slog.DebugContext(errCTX, "got telegram API response", "recipientID", recipientID, "response", response)

			if !response.Ok {
				return fmt.Errorf("request was no ok for recipient ID = %d got error %d: %s", recipientID, response.ErrorCode, response.Description)
			}

			return nil
		})
	}

	return g.Wait()
}

// escapeMarkdownV2 escapes special characters for MarkdownV2 parsing in Telegram.
func escapeMarkdownV2(text string) string {
	var builder strings.Builder
	for _, r := range text {
		switch r {
		case '_', '*', '[', ']', '(', ')', '~', '`', '>', '#', '+', '-', '=', '|', '{', '}', '.', '!':
			builder.WriteRune('\\')
		}
		builder.WriteRune(r)
	}
	return builder.String()
}

// SetupDefaultLogger configures the default slog with the envvar defined log level and mode.
func SetupDefaultLogger() error {
	level := new(slog.LevelVar)
	levelStr := os.Getenv("WOD_RAE_LOG_LEVEL")

	if levelStr != "" {
		err := level.UnmarshalText([]byte(levelStr))
		if err != nil {
			return fmt.Errorf("invalid WOD_RAE_LOG_LEVEL. Got \"%s\", expected one of \"DEBUG\", \"INFO\", \"WARN\" OR \"ERROR\"", levelStr)
		}
	} else {
		level.Set(slog.LevelInfo)
	}

	logMode := strings.ToLower(os.Getenv("WOD_RAE_LOG_MODE"))
	var handler slog.Handler

	switch logMode {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	case "text", "":
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	default:
		return fmt.Errorf("invalid WOD_RAE_LOG_MODE. Got \"%s\", expected \"text\" or \"json\"", logMode)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return nil
}
