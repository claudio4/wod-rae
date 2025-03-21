# Palabra del día RAE Telegram Bot

This project is a Telegram bot that fetches the word of the day from the Real Academia Española (RAE) website and sends it with its definitions to a list of specified Telegram users.

## How it Works

The bot scrapes the RAE website (`https://dle.rae.es/`) to find the word of the day and its definitions. Once retrieved, it formats a message containing the word and its definitions and sends it to the Telegram user IDs provided in the configuration. **Please note that the bot is designed to run once, send the word of the day, and then exit.** If you wish to run the bot on a schedule (e.g., daily), you will need to use an external scheduling tool like `systemd-timers` or `cron`.

## Getting Started

### Prerequisites

- **Telegram Bot Token:** You need to create a Telegram bot and obtain its API token. You can do this by talking to the BotFather on Telegram.
- **Telegram User IDs:** You need the Telegram user IDs of the recipients who should receive the word of the day.

### Running the Bot

The recommended way to run this bot is using Docker.

#### Docker

1. **Pull the Docker image:**

   ```bash
   docker pull ghcr.io/claudio4/wod-rae:latest
   ```

2. **Run the Docker container:**
   You need to provide the required environment variables when running the container.

   ```bash
   docker run --rm --name wod-rae-bot \
      -e WOD_RAE_BOT_TOKEN="YOUR_TELEGRAM_BOT_TOKEN" \
      -e WOD_RAE_RECIPIENTS="USER_ID_1 USER_ID_2 ... USER_ID_N" \
      ghcr.io/claudio4/wod-rae:latest
   ```

   Replace `YOUR_TELEGRAM_BOT_TOKEN` with your actual Telegram bot token and `USER_ID_1 USER_ID_2 ... USER_ID_N` with the Telegram user IDs of the recipients, separated by spaces.

#### Environment Variables

The following environment variables are required to configure the bot:

- `WOD_RAE_BOT_TOKEN`: **Required.** Your Telegram bot's API token. This is used to communicate with the Telegram API to send messages.
- `WOD_RAE_RECIPIENTS`: **Required.** A space-separated list of Telegram user IDs who will receive the word of the day.

The following environment variables are optional:

- `WOD_RAE_LOG_LEVEL`: Specifies the logging level. Possible values are `DEBUG`, `INFO`, `WARN`, or `ERROR`. Defaults to `INFO` if not set.
- `WOD_RAE_LOG_MODE`: Specifies the logging mode. Possible values are `text` or `json`. Defaults to `text` if not set or if an invalid value is provided.

## Project Structure

The main logic of the bot is implemented in Go in the `main.go` file.

## Disclaimer

This bot scrapes the Real Academia Española website. Please be aware of the terms of service and robots.txt of the RAE website before running this bot frequently.
