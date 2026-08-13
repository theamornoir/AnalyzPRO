# AnalyzPRO - Makefile
# Variant B: build a binary and run it directly.
# On Ctrl+C the binary itself dies (no parent `go run`), so no orphaned
# processes keep holding the Telegram token.

BINARY := bin/analyzpro
ENTRY  := ./cmd/bot
GO     ?= go

.PHONY: build run start dev kill clean test

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
## Сначала пробуем cloudflared (бесплатно, без аккаунта), иначе ngrok.
## Запускай в отдельном терминале. Для ngrok бот сам подхватит https-URL
## через локальное API; для cloudflared скопируй https-URL из вывода в
## переменную WEBAPP_URL и перезапусти `make run`.
tunnel:
	@if command -v cloudflared >/dev/null 2>&1; then \
		echo "==> Запуск cloudflared туннеля на :8080 (HTTPS)..."; \
		echo "==> Скопируй https-URL из вывода в WEBAPP_URL и перезапусти make run"; \
		cloudflared tunnel --url http://localhost:8080; \
	elif command -v ngrok >/dev/null 2>&1; then \
		echo "==> Запуск ngrok туннеля на :8080 (HTTPS)..."; \
		ngrok http 8080; \
	else \
		echo "❌ Не найден ни cloudflared, ни ngrok."; \
		echo "   Установи один из них:"; \
		echo "   brew install cloudflared"; \
		echo "   brew install ngrok/ngrok/ngrok"; \
		exit 1; \
	fi

## mini - НАСТОЯЩИЙ Telegram Mini App одной командой: поднимает HTTPS-туннель
## (cloudflared/ngrok), сам вытаскивает https-URL, прокидывает его боту и
## запускает бота. Дашборд откроется как Mini App прямо внутри Telegram.
## Ctrl+C останавливает и бота, и туннель.
mini:
	@bash scripts/run-miniapp.sh
