#!/usr/bin/env bash
#
# run-miniapp.sh — поднимает HTTPS-туннель (cloudflared или ngrok) к локальному
# HTTP-серверу бота и запускает бота с WEBAPP_URL=<https-url>/dashboard, чтобы
# дашборд открывался как НАСТОЯЩИЙ Telegram Mini App прямо внутри Telegram
# (а не просто ссылкой в браузере). Telegram Web App требует HTTPS, поэтому
# локальные http://localhost/ЛАН-IP не годятся для встроенной кнопки Mini App.
#
# Использование:
#   make mini            # одной командой: туннель + бот
#   HTTP_ADDR=:8080 bash scripts/run-miniapp.sh
#
set -uo pipefail

PORT="${HTTP_ADDR:-:8080}"
PORT="${PORT#:}"   # убираем ведущий ':' -> "8080"

BINARY="bin/analyzpro"
TUNNEL_URL_FILE=".tunnel_url"
LOG="$(mktemp /tmp/analyzpro-tunnel.XXXXXX.log)"

cleanup() {
  echo ""
  echo "==> Останавливаю туннель..."
  [ -n "${CF_PID:-}" ] && kill "$CF_PID" 2>/dev/null || true
  [ -n "${NGROK_PID:-}" ] && kill "$NGROK_PID" 2>/dev/null || true
  rm -f "$LOG" "$TUNNEL_URL_FILE"
}
trap cleanup EXIT INT TERM

# Извлечь первый https-URL туннеля из лог-файла.
detect_url() {
  grep -oE "https://[a-zA-Z0-9][a-zA-Z0-9.-]*\.(trycloudflare\.com|ngrok\.io|ngrok-free\.app)" "$1" 2>/dev/null | head -n1
}

echo "==> Building bot -> $BINARY ..."
go build -o "$BINARY" ./cmd/bot || { echo "❌ Ошибка сборки"; exit 1; }

URL=""
if command -v cloudflared >/dev/null 2>&1; then
  echo "==> Запуск cloudflared туннеля на :$PORT (HTTPS)..."
  cloudflared tunnel --url "http://localhost:$PORT" --no-autoupdate >"$LOG" 2>&1 &
  CF_PID=$!
  for _ in $(seq 1 45); do
    u="$(detect_url "$LOG")"
    [ -n "$u" ] && { URL="$u"; break; }
    # клафларед упал?
    if ! kill -0 "$CF_PID" 2>/dev/null; then break; fi
    sleep 1
  done
elif command -v ngrok >/dev/null 2>&1; then
  echo "==> Запуск ngrok туннеля на :$PORT (HTTPS)..."
  ngrok http "$PORT" --log=stdout >"$LOG" 2>&1 &
  NGROK_PID=$!
  for _ in $(seq 1 45); do
    u="$(detect_url "$LOG")"
    [ -n "$u" ] && { URL="$u"; break; }
    if ! kill -0 "$NGROK_PID" 2>/dev/null; then break; fi
    sleep 1
  done
else
  # Ни одной утилиты нет — пробуем автоустановить cloudflared через brew,
  # чтобы `make mini` работал одной командой. Иначе — понятная инструкция.
  if command -v brew >/dev/null 2>&1; then
    echo "⚠️  cloudflared/ngrok не найдены. Устанавливаю cloudflared через brew..."
    if brew install cloudflared; then
      echo "==> cloudflared установлен, перезапускаю..."
      exec bash "$0"
    fi
    echo "❌ Не удалось установить cloudflared через brew."
  else
    echo "❌ Не найден ни cloudflared, ни ngrok, и нет brew для установки."
  fi
  echo "   Установи туннель вручную:"
  echo "     brew install cloudflared"
  echo "     brew install ngrok/ngrok/ngrok"
  rm -f "$LOG"
  exit 1
fi

if [ -z "$URL" ]; then
  echo "❌ Не удалось получить HTTPS-URL туннеля. Лог cloudflared/ngrok:"
  cat "$LOG"
  rm -f "$LOG"
  exit 1
fi

# Прокидываем боту HTTPS-URL — теперь кнопка «Mini App» реально заработает.
export WEBAPP_URL="$URL/dashboard"
export DASHBOARD_URL="$URL/dashboard"
printf '%s/dashboard' "$URL" > "$TUNNEL_URL_FILE"

# Убиваем старые инстансы бота, чтобы не боролись за токен/порт.
echo "==> Останавливаю старые инстансы бота..."
pkill -f "bin/analyzpro" 2>/dev/null || true
pkill -f "go-build/.*-d/main" 2>/dev/null || true
lsof -ti :8080 2>/dev/null | xargs kill 2>/dev/null || true
sleep 1

echo "==> 🌐 Туннель готов: $URL"
echo "==> 🤖 WEBAPP_URL=$WEBAPP_URL  (настоящий Mini App в Telegram)"
echo "==> Запуск бота... (Ctrl+C — остановит и бота, и туннель)"
"$BINARY"
