package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/gocolly/colly/v2"
)

type runError struct {
	err      error
	exitCode int
	msg      string
}

func (e *runError) Error() string {
	if e.err == nil {
		return e.msg
	}
	if e.msg == "" {
		return e.err.Error()
	}

	return fmt.Sprintf("%s: %v", e.msg, e.err)
}

type Word struct {
	word        string
	definitions []string
}

func run(
	getenv func(string) string,
	stdout io.Writer,
) error {
	frontCollector := colly.NewCollector(
		colly.AllowedDomains("dle.rae.es"),
	)
	wordCollector := frontCollector.Clone()

	programLevel := new(slog.LevelVar)
	slog.SetDefault(slog.New(slog.NewTextHandler(stdout, &slog.HandlerOptions{Level: programLevel})))
	programLevel.Set(slog.LevelDebug)

	var wordOfTheDay Word

	frontCollector.OnHTML("a[href].c-word-day__link", func(h *colly.HTMLElement) {
		url := h.Request.AbsoluteURL(h.Attr("href"))
		slog.Debug("found word URL", "url", url)
		wordCollector.Visit(url)
	})

	frontCollector.OnHTML("span.c-word-day__word", func(h *colly.HTMLElement) {
		wordOfTheDay.word = h.Text
		slog.Info("found word of the day", "word", h.Text)
	})

	wordCollector.OnHTML("div.c-definitions__item", func(h *colly.HTMLElement) {
		slog.Debug("found word definition", "definition", h.Text)
	})

	frontCollector.Visit("https://dle.rae.es/")

	return nil
}

func main() {
	err := run(
		os.Getenv,
		os.Stdout,
	)

	if err == nil {
		return
	}

	fmt.Fprintln(os.Stderr, err)

	rErr, ok := err.(*runError)
	if !ok {
		os.Exit(1)
	}

	os.Exit(rErr.exitCode)
}
