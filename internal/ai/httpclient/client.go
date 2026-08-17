package httpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

var AIHTTPClient *http.Client

// explicitProxy - явно заданный egress-прокси ТОЛЬКО для AI-вызовов
// (Gemini/DeepSeek/Claude). Задаётся через config.GeminiProxy (.env
// GEMINI_PROXY) либо env-переменными GEMINI_PROXY / AI_HTTP_PROXY.
// Если пуст - транспорт сам подхватывает системный прокси через
// http.ProxyFromEnvironment (HTTP_PROXY / HTTPS_PROXY / NO_PROXY), а при его
// отсутствии работает напрямую.
//
// Это решает гео-блок Gemini (например, "User location is not supported" во
// Франции): включите VPN/прокси (Clash/Proxifier, локальный SOCKS5-порт) и
// укажите его в GEMINI_PROXY - трафик Telegram и всё остальное прокси
// НЕ затрагиваются.
var explicitProxy string

func init() {
	// Фоллбэк для тестов/standalone: читаем env прямо здесь, до config.Load().
	explicitProxy = resolveExplicitProxy("")
	AIHTTPClient = newAIClient()
	logProxyMode()
}

// Configure применяет явный прокси из конфигурации (config.GeminiProxy).
// Вызывается из app.New() ПОСЛЕ config.Load(), чтобы GEMINI_PROXY из .env имел
// приоритет над системным прокси и над env-фоллбэком init().
// Если proxy пуст - оставляем выбранное в init() поведение (env-фоллбэк или
// системный прокси).
func Configure(proxy string) {
	if proxy == "" {
		logProxyMode()
		return
	}
	explicitProxy = proxy
	AIHTTPClient = newAIClient()
	logProxyMode()
}

// resolveExplicitProxy возвращает первый заданный явный прокси по приоритету:
// cfgProxy → GEMINI_PROXY → AI_HTTP_PROXY. Пустая строка = явный прокси не
// задан (будем использовать системный через http.ProxyFromEnvironment).
func resolveExplicitProxy(cfgProxy string) string {
	if cfgProxy != "" {
		return cfgProxy
	}
	if v := os.Getenv("GEMINI_PROXY"); v != "" {
		return v
	}
	return os.Getenv("AI_HTTP_PROXY")
}

// newAIClient строит *http.Client с прокси-совместимым транспортом.
//  1. Если explicitProxy задан - используем его (http/https через CONNECT,
//     socks5 через кастомный дайалер).
//  2. Иначе - системный прокси (http.ProxyFromEnvironment); если его нет -
//     прямое соединение.
func newAIClient() *http.Client {
	transport := &http.Transport{
		Proxy: proxyResolver,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          50,
	}

	// SOCKS5 не поддерживается stdlib-транспортом как схема ProxyURL, поэтому
	// для него подменяем DialContext на туннель до цели (поверх - TLS), а
	// Proxy оставляем nil (запрос идёт напрямую к цели через туннель).
	if explicitProxy != "" {
		if pu, err := url.Parse(explicitProxy); err == nil && pu.Scheme == "socks5" {
			baseDial := transport.DialContext
			transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				conn, err := dialThroughSOCKS5(pu.Host, network, addr)
				if err != nil {
					log.Printf("⚠️ SOCKS5 proxy %s failed (%v), falling back to direct", redactProxy(pu), err)
					return baseDial(ctx, network, addr)
				}
				return conn, nil
			}
		}
	}

	return &http.Client{
		Transport: transport,
		Timeout:   45 * time.Second,
	}
}

// proxyResolver - функция транспорта: выбирает прокси на каждый запрос.
//  1. explicitProxy задан (http/https) → отдаём его (CONNECT-прокси).
//  2. explicitProxy = socks5 → проксируем сами через DialContext, поэтому
//     Proxy не ставим (возвращаем nil).
//  3. explicitProxy пуст → системный прокси через http.ProxyFromEnvironment
//     (подхватывает HTTP_PROXY/HTTPS_PROXY/NO_PROXY), при отсутствии -
//     прямое соединение.
func proxyResolver(req *http.Request) (*url.URL, error) {
	if explicitProxy != "" {
		pu, err := url.Parse(explicitProxy)
		if err != nil {
			log.Printf("⚠️ explicit proxy %q invalid (%v), falling back to system/direct", redactProxyStr(explicitProxy), err)
		} else if pu.Scheme == "socks5" {
			// socks5 проксируем через DialContext - Proxy не ставим
			return nil, nil
		} else {
			return pu, nil
		}
	}
	return http.ProxyFromEnvironment(req)
}

// logProxyMode выводит, через что пойдёт AI-трафик. По логам сразу видно,
// работает прокси или соединение напрямую (диагностика гео-блока Gemini).
func logProxyMode() {
	if explicitProxy != "" {
		if pu, err := url.Parse(explicitProxy); err == nil {
			log.Printf("✅ Gemini client using proxy: %s", redactProxy(pu))
		} else {
			log.Printf("✅ Gemini client using proxy: %s", redactProxyStr(explicitProxy))
		}
		return
	}
	// Что реально увидит http.ProxyFromEnvironment для Gemini-хоста?
	probe := &http.Request{URL: &url.URL{Scheme: "https", Host: "generativelanguage.googleapis.com"}}
	if u, err := http.ProxyFromEnvironment(probe); err == nil && u != nil {
		log.Printf("✅ Gemini client using proxy: %s (system HTTP_PROXY/HTTPS_PROXY)", redactProxy(u))
		return
	}
	log.Printf("⚠️ Gemini client using DIRECT connection (no proxy)")
}

// redactProxy скрывает учётные данные прокси в логах (user:pass@host).
func redactProxy(u *url.URL) string {
	if u == nil {
		return ""
	}
	host := u.Host
	if u.User != nil {
		if _, hasPass := u.User.Password(); hasPass {
			host = "***:***@" + u.Host
		} else if u.User.Username() != "" {
			host = "***@" + u.Host
		}
	}
	return u.Scheme + "://" + host
}

// redactProxyStr - обёртка для сырой строки прокси.
func redactProxyStr(s string) string {
	if u, err := url.Parse(s); err == nil {
		return redactProxy(u)
	}
	return s
}

func FetchWithRetry(ctx context.Context, url string, body io.Reader, maxRetries int) ([]byte, error) {
	var lastErr error
	backoff := 2 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := AIHTTPClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetries {
				time.Sleep(backoff)
				backoff *= 2
				continue
			}
			return nil, err
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			if attempt < maxRetries {
				time.Sleep(backoff)
				backoff *= 2
				continue
			}
			return nil, err
		}

		// 4xx (кроме 429) - клиентские ошибки (400/401/403/404), повторять
		// бесполезно. 429 (rate limit) и 5xx повторяем с эксп. отступом.
		if resp.StatusCode >= 400 && resp.StatusCode < 600 {
			retryable := resp.StatusCode == http.StatusTooManyRequests ||
				(resp.StatusCode >= 500 && resp.StatusCode < 600)
			if retryable && attempt < maxRetries {
				time.Sleep(backoff)
				backoff *= 2
				continue
			}
			if resp.StatusCode == http.StatusTooManyRequests {
				return nil, &HTTPError{StatusCode: resp.StatusCode, Message: "rate limit exceeded"}
			}
			return nil, &HTTPError{StatusCode: resp.StatusCode, Message: string(respBody)}
		}

		return respBody, nil
	}

	return nil, lastErr
}

type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return e.Message
}

// dialThroughSOCKS5 устанавливает TCP-туннель через SOCKS5-прокси (только
// метод "no authentication") и возвращает net.Conn, уже подключённый к target.
// Поверх этого соединения http.Transport сам сделает TLS, поэтому работает и
// для https-целей (Gemini/DeepSeek/Claude). Зависимостей нет - только stdlib.
func dialThroughSOCKS5(proxyAddr, network, addr string) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", proxyAddr, 5*time.Second)
	if err != nil {
		return nil, err
	}

	// 1. Greeting: VER=5, NMETHODS=1, METHOD=0x00 (no auth)
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		conn.Close()
		return nil, err
	}
	greet := make([]byte, 2)
	if _, err := io.ReadFull(conn, greet); err != nil {
		conn.Close()
		return nil, err
	}
	if greet[0] != 0x05 || greet[1] != 0x00 {
		conn.Close()
		return nil, fmt.Errorf("socks5 handshake rejected: %v", greet)
	}

	// 2. CONNECT-запрос
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		conn.Close()
		return nil, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		conn.Close()
		return nil, err
	}

	var req bytes.Buffer
	req.WriteByte(0x05) // VER
	req.WriteByte(0x01) // CMD = CONNECT
	req.WriteByte(0x00) // RSV
	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			req.WriteByte(0x01) // ATYP = IPv4
			req.Write(ip4)
		} else {
			req.WriteByte(0x04) // ATYP = IPv6
			req.Write(ip.To16())
		}
	} else {
		req.WriteByte(0x03) // ATYP = domain
		req.WriteByte(byte(len(host)))
		req.WriteString(host)
	}
	req.WriteByte(byte(port >> 8))
	req.WriteByte(byte(port & 0xff))

	if _, err := conn.Write(req.Bytes()); err != nil {
		conn.Close()
		return nil, err
	}

	// 3. Ответ: VER REP RSV ATYP + bind-addr (пропускаем)
	hdr := make([]byte, 4)
	if _, err := io.ReadFull(conn, hdr); err != nil {
		conn.Close()
		return nil, err
	}
	if hdr[1] != 0x00 {
		conn.Close()
		return nil, fmt.Errorf("socks5 connect failed, rep=%d", hdr[1])
	}
	switch hdr[3] {
	case 0x01: // IPv4
		io.ReadFull(conn, make([]byte, 4+2))
	case 0x04: // IPv6
		io.ReadFull(conn, make([]byte, 16+2))
	case 0x03: // domain
		l := make([]byte, 1)
		io.ReadFull(conn, l)
		io.ReadFull(conn, make([]byte, int(l[0])+2))
	}

	return conn, nil
}
