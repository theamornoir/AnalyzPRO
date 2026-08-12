package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	BotToken           string
	GoogleGeminiAPIKey string
	GoogleAIModel      string
	UploadDir          string
	AppEnv             string
	LoadingStickerID   string
	AdminChatID        int64
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	adminID, _ := strconv.ParseInt(os.Getenv("ADMIN_CHAT_ID"), 10, 64)

	return &Config{
		BotToken:           os.Getenv("BOT_TOKEN"),
		GoogleGeminiAPIKey: os.Getenv("GOOGLE_GEMINI_API_KEY"),
		GoogleAIModel:      getEnv("GOOGLE_AI_MODEL", "gemini-3.6-flash"),
		UploadDir:          getEnv("UPLOAD_DIR", "./uploads"),
		AppEnv:             getEnv("APP_ENV", "development"),
		LoadingStickerID:   os.Getenv("LOADING_STICKER_ID"),
		AdminChatID:        adminID,
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
