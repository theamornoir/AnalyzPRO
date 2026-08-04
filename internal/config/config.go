package config

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	AppEnv            string
	BotToken          string
	GoogleGeminiAPIKey string
	GoogleAIModel     string
	DatabaseURL       string
	UploadDir         string
	LogLevel          string
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	viper.AutomaticEnv()

	cfg := &Config{
		AppEnv:             viper.GetString("APP_ENV"),
		BotToken:           viper.GetString("BOT_TOKEN"),
		GoogleGeminiAPIKey: viper.GetString("GOOGLE_GEMINI_API_KEY"),
		GoogleAIModel:      viper.GetString("GOOGLE_AI_MODEL"),
		DatabaseURL:        viper.GetString("DATABASE_URL"),
		UploadDir:          viper.GetString("UPLOAD_DIR"),
		LogLevel:           viper.GetString("LOG_LEVEL"),
	}

	return cfg, nil
}
