package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramBotToken     string
	DatabaseURL          string
	AnthropicAPIKey      string
	Timezone             string
	DefaultReminderHour  int
	SchedulerIntervalSec int
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		AnthropicAPIKey:  os.Getenv("ANTHROPIC_API_KEY"),
		Timezone:         os.Getenv("TIMEZONE"),
	}

	if cfg.TelegramBotToken == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.AnthropicAPIKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY is required")
	}
	if cfg.Timezone == "" {
		cfg.Timezone = "Asia/Jakarta"
	}

	if v := os.Getenv("DEFAULT_REMINDER_HOUR"); v != "" {
		h, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid DEFAULT_REMINDER_HOUR: %w", err)
		}
		cfg.DefaultReminderHour = h
	} else {
		cfg.DefaultReminderHour = 7
	}

	if v := os.Getenv("SCHEDULER_INTERVAL_SEC"); v != "" {
		s, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid SCHEDULER_INTERVAL_SEC: %w", err)
		}
		cfg.SchedulerIntervalSec = s
	} else {
		cfg.SchedulerIntervalSec = 30
	}

	return cfg, nil
}
