package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

func main() {
	apiKey := os.Getenv("GOOGLE_GEMINI_API_KEY")
	if apiKey == "" {
		// Если переменная не установлена, пробуем прочитать из .env
		fmt.Println("⚠️ GOOGLE_GEMINI_API_KEY not set. Please set it first:")
		fmt.Println("export GOOGLE_GEMINI_API_KEY=your_key_here")
		os.Exit(1)
	}

	fmt.Printf("🔑 Using API Key: %s...%s\n", apiKey[:10], apiKey[len(apiKey)-4:])
	fmt.Println("📡 Fetching available models...")
	fmt.Println()

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models?key=%s", apiKey)

	resp, err := http.Get(url)
	if err != nil {
		log.Fatal("❌ Network error:", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("❌ Failed to read response:", err)
	}

	var result struct {
		Models []struct {
			Name             string   `json:"name"`
			DisplayName      string   `json:"displayName"`
			Description      string   `json:"description"`
			SupportedMethods []string `json:"supportedMethods"`
		} `json:"models"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println("❌ Failed to parse response:")
		fmt.Println(string(body))
		log.Fatal(err)
	}

	fmt.Println("📋 ДОСТУПНЫЕ МОДЕЛИ:")
	fmt.Println("====================")
	fmt.Println()

	found := false
	for _, model := range result.Models {
		// Проверяем, поддерживает ли модель generateContent
		supportsGenerate := false
		for _, method := range model.SupportedMethods {
			if method == "generateContent" {
				supportsGenerate = true
				break
			}
		}

		if supportsGenerate {
			found = true
			fmt.Printf("✅ %s\n", model.Name)
			fmt.Printf("   📝 Название: %s\n", model.DisplayName)
			if len(model.Description) > 0 {
				desc := model.Description
				if len(desc) > 100 {
					desc = desc[:100] + "..."
				}
				fmt.Printf("   📖 Описание: %s\n", desc)
			}
			fmt.Println()
		}
	}

	if !found {
		fmt.Println("❌ Нет моделей с поддержкой generateContent")
		fmt.Println("Полный ответ API:")
		fmt.Println(string(body))
	}
}
