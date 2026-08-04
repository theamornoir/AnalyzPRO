package config

import (
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	AppEnv       string
	BotToken     string
	OpenAIAPIKey string
	DatabaseURL  string
	UploadDir    string
	LogLevel     string
}

func Load() (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		return nil, err
	}

	viper.AutomaticEnv()

	cfg := &Config{
		AppEnv:       viper.GetString("APP_ENV"),
		BotToken:     viper.GetString("BOT_TOKEN"),
		OpenAIAPIKey: viper.GetString("OPENAI_API_KEY"),
		DatabaseURL:  viper.GetString("DATABASE_URL"),
		UploadDir:    viper.GetString("UPLOAD_DIR"),
		LogLevel:     viper.GetString("LOG_LEVEL"),
	}

	return cfg, nil
}
