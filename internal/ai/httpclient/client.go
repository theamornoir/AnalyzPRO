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

func init() {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          50,
	}

	// AI_HTTP_PROXY — необязательный egress-прокси ТОЛЬКО для AI-вызовов
	// (Gemini/DeepSeek/Claude). Полезно, когда исходящий IP сервера попадает под
	// гео-блок (например, "User location is not supported" от Gemini). При этом
	// трафик Telegram и всё остальное прокси НЕ затрагиваются.
	// Поддерживаются http/https-прокси и socks5:// (например, локальный порт
	// VPN/Clash-клиента: socks5://127.0.0.1:7891).
	if proxyURL := os.Getenv("AI_HTTP_PROXY"); proxyURL != "" {
		pu, err := url.Parse(proxyURL)
		if err != nil {
			log.Printf("AI_HTTP_PROXY invalid, ignored: %v", err)
		} else {
			switch pu.Scheme {
			case "http", "https":
				transport.Proxy = http.ProxyURL(pu)
				log.Printf("AI HTTP client using HTTP proxy: %s", pu.Host)
			case "socks5":
				transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
					return dialThroughSOCKS5(pu.Host, network, addr)
				}
				log.Printf("AI HTTP client using SOCKS5 proxy: %s", pu.Host)
			default:
				log.Printf("AI_HTTP_PROXY unsupported scheme %q, ignored", pu.Scheme)
			}
		}
	}

	AIHTTPClient = &http.Client{
		Transport: transport,
		Timeout:   45 * time.Second,
	}

	log.Printf("HTTP client initialized (timeout=45s, MaxIdleConns=50)")
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

		if resp.StatusCode >= 400 && resp.StatusCode < 600 {
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
// для https-целей (Gemini/DeepSeek/Claude). Зависимостей нет — только stdlib.
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
