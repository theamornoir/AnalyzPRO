#!/usr/bin/env bash
#
# mini-stop.sh — останавливает запущенный `make mini` ИЗ ЛЮБОГО терминала.
# Убивает оболочку скрипта (по lock-PID), туннель (ngrok/cloudflared) и бота,
# освобождает порт 8080 и снимает lock-каталог. Не нужно искать тот терминал,
# где крутится `make mini` — это можно вызвать откуда угодно.
#
# Использование:
#   make mini-stop
#   bash scripts/mini-stop.sh
#
set -uo pipefail

LOCKDIR="/tmp/analyzpro-mini.lock"
LOCKPID="$LOCKDIR/pid"

echo "==> Останавливаю запущенный 'make mini'..."

# 1) Завершаем оболочку скрипта по PID из lock — её trap EXIT сам добьёт
#    туннель/бота и снимет lock.
if [ -f "$LOCKPID" ]; then
  P="$(cat "$LOCKPID" 2>/dev/null)"
  if [ -n "$P" ] && kill -0 "$P" 2>/dev/null; then
    echo "==> Завершаю оболочку run-miniapp.sh (PID $P)..."
    kill "$P" 2>/dev/null || true
    for _ in $(seq 1 5); do
      kill -0 "$P" 2>/dev/null || break
      sleep 1
    done
    # Если trap не сработал (принудительно убит) — добиваем жёстко.
    kill -9 "$P" 2>/dev/null || true
  else
    echo "==> Lock-PID '$P' уже мёртв — устаревший lock, снимаю."
  fi
fi

# 2) На всякий случай добиваем осиротевшие процессы напрямую (если скрипт
#    был убит терминалом и его trap не отработал).
echo "==> Добиваю осиротевшие ngrok/cloudflared/бот..."
pkill -x ngrok 2>/dev/null || true
pkill -x cloudflared 2>/dev/null || true
pkill -f "bin/analyzpro" 2>/dev/null || true
pkill -f "ngrok http" 2>/dev/null || true
# Освобождаем порт 8080 (HTTP-сервер бота).
lsof -ti :8080 2>/dev/null | xargs kill 2>/dev/null || true

# 3) Снимаем lock-каталог, если скрипт его не убрал сам.
rm -rf "$LOCKDIR" 2>/dev/null || true

sleep 1

echo "==> Статус:"
if pgrep -x ngrok >/dev/null 2>&1 || pgrep -f "bin/analyzpro" >/dev/null 2>&1; then
  echo "   ⚠️  Кое-что ещё живо:"
  pgrep -fa "ngrok|bin/analyzpro|cloudflared" 2>/dev/null || true
else
  echo "   ✅ Всё остановлено. Порт 8080 свободен, lock снят."
fi
