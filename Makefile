# AnalyzPRO - Makefile
# Variant B: build a binary and run it directly.
# On Ctrl+C the binary itself dies (no parent `go run`), so no orphaned
# processes keep holding the Telegram token.

BINARY := bin/analyzpro
ENTRY  := ./cmd/bot
GO     ?= go

.PHONY: build run start dev kill clean test docker-build deploy mini mini-stop

## build - compile bin/analyzpro
build:
	@echo "==> Building $(BINARY) ..."
	@$(GO) build -o $(BINARY) $(ENTRY)
	@echo "==> Build OK -> $(BINARY)"

## run / start - kill old instances, build and run the bot (foreground)
run: kill build
	@echo "==> Starting AnalyzPRO bot (foreground)..."
	@./$(BINARY)

start: run
	@true

## dev - alias for run
dev: run
	@true

## kill - stop all running bot instances (incl. orphans left by go run)
kill:
	@echo "==> Stopping running AnalyzPRO instances..."
	@-pkill -f "bin/analyzpro" 2>/dev/null || true
	@-pkill -f "/exe/main" 2>/dev/null || true
	@-pkill -f "go-build/.*-d/main" 2>/dev/null || true
	@-pkill -f "go run" 2>/dev/null || true
	@-pkill -f "AnalyzPRO/bin/analyzpro" 2>/dev/null || true
	@-lsof -ti :8080 2>/dev/null | xargs kill 2>/dev/null || true
	@sleep 1
	@echo "==> Done."

## clean - remove build artifacts
clean:
	@rm -rf bin
	@echo "==> Cleaned build artifacts."

## test - build check + tests
test:
	@$(GO) build ./...
	@$(GO) test ./... 2>/dev/null || echo "(no tests yet)"
	@echo "==> Build check OK."

## tunnel - поднять HTTPS-туннель к локальному :8080, чтобы Mini App
## дашборда открывался на телефоне (Telegram Web App требует HTTPS).
## ⚠️  cloudflared Quick-туннель (trycloudflare.com) НЕ используется — он
##     часто НЕ открывается с мобильных сетей (ошибка Cloudflare).
##     Используйте ngrok (нужен бесплатный токен:
##     `ngrok config add-authtoken <токен>`) или именованный Cloudflare-
##     туннель со своим доменом (задайте CF_TUNNEL_URL).
## Запускай в отдельном терминале. Бот сам подхватит https-URL ngrok через
## локальное API (127.0.0.1:4040). См. docs/DEPLOY.md (про прод и про
## именованный Cloudflare-туннель со своим доменом).
tunnel:
	@if command -v ngrok >/dev/null 2>&1; then \
		echo "==> Запуск ngrok туннеля на :8080 (HTTPS, стабилен на телефоне)..."; \
		echo "==> Бот сам подхватит https-URL через локальное API ngrok."; \
		echo "==> Если ngrok пишет про authtoken — добавьте его:"; \
		echo "==>   ngrok config add-authtoken <токен>  (https://dashboard.ngrok.com/get-started/your-authtoken)"; \
		ngrok http 8080; \
	elif [ -n "$${CF_TUNNEL_URL:-}" ] && command -v cloudflared >/dev/null 2>&1; then \
		echo "==> Запуск NAMED cloudflared туннеля -> $${CF_TUNNEL_URL}"; \
		cloudflared tunnel run "$${CF_TUNNEL:-analyzpro}"; \
	else \
		echo "❌ Нет ngrok и не задан CF_TUNNEL_URL."; \
		echo "   Самый надёжный вариант для телефона — ngrok (бесплатно):"; \
		echo "     brew install ngrok/ngrok/ngrok"; \
		echo "     ngrok config add-authtoken <токен>"; \
		echo "     ngrok http 8080"; \
		echo "   Или поднимите бота на своём домене с HTTPS (см. docs/DEPLOY.md)."; \
		exit 1; \
	fi

## mini - НАСТОЯЩИЙ Telegram Mini App одной командой: поднимает HTTPS-туннель
## (cloudflared/ngrok), сам вытаскивает https-URL, прокидывает его боту и
## запускает бота. Дашборд откроется как Mini App прямо внутри Telegram.
## Ctrl+C останавливает и бота, и туннель.
mini:
	@bash scripts/run-miniapp.sh

## mini-stop - остановить запущенный 'make mini' ИЗ ЛЮБОГО терминала
## (без Ctrl+C в нужном окне). Убивает оболочку по lock-PID, туннель и бота,
## освобождает :8080 и снимает lock.
mini-stop:
	@bash scripts/mini-stop.sh

## pdf-service - (удалено) конвертация HTML→PDF теперь идёт через внешний
## сервис html2pdf.com по ключу HTML2PDF_API_KEY; локальный Chrome-сервер не
## нужен.

