package config

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	BotToken           string
	GoogleGeminiAPIKey string
	GoogleAIModel      string
	DeepSeekAPIKey     string
	AnthropicAPIKey    string
	UploadDir          string
	AppEnv             string
	LoadingStickerID   string
	AdminChatID        int64
	WebAppURL          string
	DashboardURL       string
	HTTPAddr           string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	adminID, _ := strconv.ParseInt(os.Getenv("ADMIN_CHAT_ID"), 10, 64)

	httpAddr := getEnv("HTTP_ADDR", ":8080")
	webAppURL := getEnv("WEBAPP_URL", "")
	if webAppURL == "" {
		// Сначала пробуем подхватить уже запущенный HTTPS-туннель
		// (ngrok/cloudflared) — тогда Web App откроется и на телефоне.
		if u := detectTunnelURL(httpAddr); u != "" {
			webAppURL = u
		} else {
			// Авто-детект: пытаемся подставить LAN-IP машины, чтобы Web App
			// был доступен с клиента в той же сети. localhost оставляем фоллбэком.
			webAppURL = defaultWebAppURL(httpAddr)
		}
	}
	dashboardURL := getEnv("DASHBOARD_URL", "")
	if dashboardURL == "" {
		// Если сконфигурированный WebApp-URL — localhost, а у машины есть
		// LAN-IP, делаем «открываемый» URL на LAN-IP. Так дашборд можно
		// открыть с телефона в той же Wi-Fi сети (по ссылке/в браузере),
		// хотя встроенная WebApp-кнопка всё равно требует HTTPS.
		if lan := lanURLFor(webAppURL); lan != "" {
			dashboardURL = lan
		} else {
			dashboardURL = webAppURL
		}
	}

	return &Config{
		BotToken:           strings.TrimSpace(os.Getenv("BOT_TOKEN")),
		GoogleGeminiAPIKey: os.Getenv("GOOGLE_GEMINI_API_KEY"),
		GoogleAIModel:      getEnv("GOOGLE_AI_MODEL", "gemini-3.6-flash"),
		DeepSeekAPIKey:     os.Getenv("DEEPSEEK_API_KEY"),
		AnthropicAPIKey:    os.Getenv("ANTHROPIC_API_KEY"),
		UploadDir:          getEnv("UPLOAD_DIR", "./uploads"),
		AppEnv:             getEnv("APP_ENV", "development"),
		LoadingStickerID:   os.Getenv("LOADING_STICKER_ID"),
		AdminChatID:        adminID,
		WebAppURL:          webAppURL,
		DashboardURL:       dashboardURL,
		HTTPAddr:           httpAddr,
	}, nil
}

// defaultWebAppURL строит URL дашборда из адреса HTTP-сервера.
// Если у машины есть LAN-IP (не loopback), подставляем его, иначе localhost.
// Для доступа с телефона через интернет всё равно нужен HTTPS-туннель
// (ngrok/cloudflared) — тогда задайте WEBAPP_URL явно.
func defaultWebAppURL(httpAddr string) string {
	port := "8080"
	if strings.HasPrefix(httpAddr, ":") {
		port = httpAddr[1:]
	} else if _, p, err := net.SplitHostPort(httpAddr); err == nil && p != "" {
		port = p
	}

	if ip := detectLANIP(); ip != "" {
		return "http://" + ip + ":" + port + "/dashboard"
	}
	return "http://localhost:" + port + "/dashboard"
}

// detectLANIP возвращает IPv4-адрес локальной сети, по которому клиенты
// (например, телефон в той же Wi-Fi сети) могут достучаться до HTTP-сервера.
// Пропускает loopback, link-local (169.254), Docker-bridge (172.16.0.0/12),
// TEST-NET (198.18.0.0/15, часто VPN/прокси), и отдаёт приоритет привычным
// домашним диапазонам (192.168.x, затем 10.x).
func detectLANIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	var found string
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() {
			continue
		}
		ip4 := ipnet.IP.To4()
		if ip4 == nil {
			continue
		}
		// link-local / APIPA
		if ip4[0] == 169 && ip4[1] == 254 {
			continue
		}
		// Docker bridge (172.16.0.0/12)
		if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
			continue
		}
		// TEST-NET-2/3 (198.18.0.0/15) — часто VPN/прокси
		if ip4[0] == 198 && ip4[1] == 18 {
			continue
		}

		s := ip4.String()
		// Самый вероятный домашний диапазон — сразу возвращаем.
		if ip4[0] == 192 && ip4[1] == 168 {
			return s
		}
		if found == "" {
			found = s
			continue
		}
		// 10.x предпочтительнее прочих (но уступает 192.168.x).
		if ip4[0] == 10 && !strings.HasPrefix(found, "192.168") {
			found = s
		}
	}
	return found
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// lanURLFor — если rawURL указывает на localhost/127.0.0.1, возвращает тот же
// URL, но с хостом, заменённым на LAN-IP машины (если он есть). Это позволяет
// открывать дашборд с телефона в той же Wi-Fi сети по обычной ссылке. Для
// не-localhost URL (например, https-туннель) возвращает "".
func lanURLFor(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := u.Hostname()
	if host != "localhost" && host != "127.0.0.1" {
		return ""
	}
	lan := detectLANIP()
	if lan == "" {
		return ""
	}
	port := u.Port()
	hostPort := lan
	if port != "" {
		hostPort = net.JoinHostPort(lan, port)
	}
	u.Host = hostPort
	return u.String()
}

// extractPort — извлекает номер порта из HTTP_ADDR (например ":8080" → "8080").
func extractPort(httpAddr string) string {
	port := "8080"
	if strings.HasPrefix(httpAddr, ":") {
		port = httpAddr[1:]
	} else if _, p, err := net.SplitHostPort(httpAddr); err == nil && p != "" {
		port = p
	}
	return port
}

// detectTunnelURL — пытается найти уже запущенный HTTPS-туннель к нашему
// HTTP-серверу (например, `ngrok http 8080`). Если найден — возвращает его
// публичный https-URL с суффиксом /dashboard. Telegram Web App требует HTTPS,
// поэтому это позволяет открывать дашборд-миниапп прямо с телефона.
//
// Поддерживается ngrok (локальное API http://127.0.0.1:4040/api/tunnels).
// Для cloudflared задайте WEBAPP_URL/DASHBOARD_URL явно в .env.
func detectTunnelURL(httpAddr string) string {
	port := extractPort(httpAddr)

	client := &http.Client{Timeout: 600 * time.Millisecond}
	resp, err := client.Get("http://127.0.0.1:4040/api/tunnels")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var data struct {
		Tunnels []struct {
			PublicURL string `json:"public_url"`
			Config    struct {
				Addr string `json:"addr"`
			} `json:"config"`
		} `json:"tunnels"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return ""
	}

	for _, t := range data.Tunnels {
		if !strings.HasPrefix(t.PublicURL, "https") {
			continue
		}
		if port != "" && strings.Contains(t.Config.Addr, port) {
			return strings.TrimRight(t.PublicURL, "/") + "/dashboard"
		}
	}

	// Fallback: если туннель был поднят отдельно (например, скриптом
	// scripts/run-miniapp.sh или make mini), он мог сохранить свой
	// https-URL в .tunnel_url — подхватим его, чтобы Web App работал как
	// настоящий Mini App в Telegram, даже если бот перезапущен.
	if data, err := os.ReadFile(".tunnel_url"); err == nil {
		s := strings.TrimSpace(string(data))
		if strings.HasPrefix(s, "https") {
			return strings.TrimRight(s, "/")
		}
	}

	return ""
}
