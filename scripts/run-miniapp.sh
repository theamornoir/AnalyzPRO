#!/usr/bin/env bash
#
# run-miniapp.sh — поднимает HTTPS-туннель к локальному HTTP-серверу бота и
# запускает бота с WEBAPP_URL=<https-url>/dashboard, чтобы дашборд открывался
# как НАСТОЯЩИЙ Telegram Mini App внутри Telegram (в том числе с телефона).
# Telegram Web App требует HTTPS, поэтому localhost/ЛАН-IP не годятся для
# встроенной кнопки Mini App.
#
# ⚠️  ПРО «Cloudflare error» на телефоне: cloudflared Quick-туннели
#     (*.trycloudflare.com) иногда НЕ открываются с мобильных сетей —
#     Cloudflare вместо страницы отдаёт экран ошибки. Поэтому ngrok —
#     самый надёжный выбор по умолчанию, а cloudflared оставлен опцией
#     (TUNNEL=cloudflared) для тех, у кого с ngrok проблемы (лимит агентов).
#
#     Порядок выбора туннеля (снизу вверх — берётся первый рабочий):
#       1. ngrok              — самый надёжный на телефоне. Нужен БЕСПЛАТНЫЙ
#                               аккаунт + authtoken (см. инструкцию ниже).
#       2. named cloudflared  — свой домен, работает и на телефоне, и в проде
#                               (задайте CF_TUNNEL + CF_TUNNEL_URL).
#       3. bore.pub           — если установлен и доступен (без аккаунта).
#       4. если ничего нет    — бот стартует с LAN-IP: дашборд откроется в
#                               БРАУЗЕРЕ телефона в той же Wi-Fi, но НЕ как
#                               встроенный Mini App (для него нужен HTTPS).
#
#     cloudflared Quick-туннель теперь ДОСТУПЕН опционально: TUNNEL=cloudflared
#     (без аккаунта и без лимита агентов). На части мобильных сетей Cloudflare
#     может выдать экран ошибки — тогда вернитесь на ngrok (TUNNEL=ngrok).
#
# Для ПРОДА туннели не нужны вообще: поднимите бота на своём домене с HTTPS
# и задайте WEBAPP_URL=https://<ваш-домен>/dashboard (см. docs/DEPLOY.md).
#
# Получить бесплатный ngrok-токен (один раз):
#   1. Зарегистрироваться: https://dashboard.ngrok.com/signup  (бесплатно)
#   2. Скопировать токен:  https://dashboard.ngrok.com/get-started/your-authtoken
#   3. Добавить:           ngrok config add-authtoken <ВАШ_ТОКЕН>
#   4. Запустить:          make mini
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
LOG="/tmp/analyzpro-tunnel-$$.log"; rm -f "$LOG"

# --- ОДИН ЭКЗЕМПЛЯР НА ВСЮ МАШИНУ (singleton через lock-каталог) ----------
# Главная причина err_ngrok_3200 — несколько одновременных `make mini` в
# разных окнах: бесплатный ngrok разрешает ровно ОДИН агент, второй падает.
# Скрипт создаёт lock-каталог (mkdir атомарен) — второй запуск не стартует,
# а честно сообщает и выходит. Это делает err_ngrok_3200 невозможным даже
# случайно. (flock на macOS нет, поэтому используем mkdir.)
LOCKDIR="/tmp/analyzpro-mini.lock"
LOCKPID="$LOCKDIR/pid"
if ! mkdir "$LOCKDIR" 2>/dev/null; then
  # Lock существует — проверяем, жив ли его владелец. Если терминал был
  # закрыт/убит (например, через terminal_stop), процесс-владелец мёртв, а
  # lock остался «висеть» — считаем его устаревшим и перехватываем запуск,
  # иначе make mini вечно блокировался бы «уже запущен».
  STALE=1
  if [ -f "$LOCKPID" ]; then
    OLD_PID="$(cat "$LOCKPID" 2>/dev/null)"
    if [ -n "$OLD_PID" ] && kill -0 "$OLD_PID" 2>/dev/null; then
      STALE=0
      echo "⚠️  Уже запущен другой экземпляр 'make mini' (PID $OLD_PID, lock: $LOCKDIR)."
    fi
  fi
  if [ "$STALE" -eq 0 ]; then
    echo "    Заверши его (Ctrl+C в том терминале) и запусти заново —"
    echo "    иначе два ngrok-агента дадут err_ngrok_3200."
    exit 1
  fi
  # Снимаем устаревший lock от убитого терминала и пробуем создать заново.
  rm -rf "$LOCKDIR" 2>/dev/null
  if ! mkdir "$LOCKDIR" 2>/dev/null; then
    echo "⚠️  Не удалось создать lock-каталог: $LOCKDIR (нет прав?)."
    exit 1
  fi
fi
echo "$$" > "$LOCKPID"

# Выбор туннеля: ngrok (по умолчанию) | cloudflared | cf-named.
TUNNEL="${TUNNEL:-ngrok}"

# --- ГАРАНТИЯ ОДНОГО ngrok-АГЕНТА (защита от err_ngrok_3200) ---------------
# Бесплатный аккаунт ngrok разрешает ровно ОДИН одновременный агент. Если
# запустить 'make mini' второй раз, пока первый ещё жив (или остался
# осиротевший ngrok от прошлого запуска), второй агент падает с
# ERR_NGROK_3200. Поэтому САМОЕ ПЕРВОЕ, что делает скрипт — убивает ЛЮБОЙ
# уже запущенный ngrok и старый бот, чтобы гарантированно остался ровно один
# агент. Это делает err_ngrok_3200 невозможным даже при повторном запуске.
echo "==> Снимаю любые старые ngrok/cloudflared/бот (защита от err_ngrok_3200)..."
pkill -x ngrok 2>/dev/null || true
pkill -x cloudflared 2>/dev/null || true
pkill -f "bin/analyzpro" 2>/dev/null || true
pkill -f "ngrok http" 2>/dev/null || true
# Даём процессам корректно завершиться и освободить порт 4040/8080.
for _ in $(seq 1 10); do
  pgrep -x ngrok >/dev/null 2>&1 || break
  sleep 1
done
sleep 1

cleanup() {
  echo ""
  echo "==> Останавливаю туннель..."
  [ -n "${CF_PID:-}" ] && kill "$CF_PID" 2>/dev/null || true
  [ -n "${NGROK_PID:-}" ] && kill "$NGROK_PID" 2>/dev/null || true
  [ -n "${BORE_PID:-}" ] && kill "$BORE_PID" 2>/dev/null || true
  rm -f "$LOG" "$TUNNEL_URL_FILE" "$LOCKPID"
  rmdir "$LOCKDIR" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

# Извлечь первый https-URL туннеля из лог-файла.
# ⚠️ ngrok периодически меняет домен бесплатного плана:
#    было *.ngrok-free.app, сейчас *.ngrok-free.dev — учитываем оба.
detect_url() {
  grep -oE "https://[a-zA-Z0-9][a-zA-Z0-9.-]*\.(trycloudflared\.com|trycloudflare\.com|ngrok\.io|ngrok-free\.(app|dev)|bore\.pub)" "$1" 2>/dev/null | head -n1
}

# Прочитать публичный https-URL из локального API ngrok (если запущен).
ngrok_url() {
  local body
  body="$(curl -s --max-time 1 "http://127.0.0.1:4040/api/tunnels" 2>/dev/null)"
  [ -z "$body" ] && return 0
  printf '%s' "$body" | grep -oE "https://[a-zA-Z0-9][a-zA-Z0-9.-]*\.ngrok-free\.(app|dev)" | head -n1
}

echo "==> Building bot -> $BINARY ..."
go build -o "$BINARY" ./cmd/bot || { echo "❌ Ошибка сборки"; exit 1; }

URL=""

# --- Вариант 1: NAMED cloudflared туннель (свой домен, без ошибок) ----------
# Если заранее настроен именованный туннель Cloudflare со своим доменом —
# он стабилен и на телефоне, и в проде. Настройка (один раз):
#   cloudflared login
#   cloudflared tunnel create analyzpro
#   cloudflared tunnel route dns analyzpro miniapp.ваш-домен.com
#   export CF_TUNNEL=analyzpro
#   export CF_TUNNEL_URL=https://miniapp.ваш-домен.com
if [ -z "$URL" ] && [ -n "${CF_TUNNEL_URL:-}" ] && command -v cloudflared >/dev/null 2>&1; then
  echo "==> Запуск NAMED cloudflared туннеля '${CF_TUNNEL:-analyzpro}' -> $CF_TUNNEL_URL"
  cloudflared tunnel run "${CF_TUNNEL:-analyzpro}" >"$LOG" 2>&1 &
  CF_PID=$!
  for _ in $(seq 1 20); do
    if curl -sf --max-time 2 "$CF_TUNNEL_URL/healthz" >/dev/null 2>&1; then break; fi
    if ! kill -0 "$CF_PID" 2>/dev/null; then break; fi
    sleep 1
  done
  if curl -sf --max-time 2 "$CF_TUNNEL_URL/healthz" >/dev/null 2>&1; then
    URL="$CF_TUNNEL_URL"
  else
    echo "⚠️  Named-туннель не поднялся. Проверьте CF_TUNNEL/CF_TUNNEL_URL и 'cloudflared tunnel list'."
  fi
fi

# --- Вариант: cloudflared Quick-туннель (без аккаунта, без лимита) --------
# НЕ имеет лимита «1 агент» (в отличие от ngrok free) — err_ngrok_3200
# невозможен в принципе. Запускается при TUNNEL=cloudflared.
# ⚠️ На части мобильных сетей Cloudflare может выдать экран ошибки — если
# увидишь, переключись на ngrok: TUNNEL=ngrok make mini.
if [ -z "$URL" ] && [ "${TUNNEL:-ngrok}" = "cloudflared" ] && command -v cloudflared >/dev/null 2>&1; then
  echo "==> Запуск cloudflared Quick-туннеля на :$PORT (HTTPS, без лимита агентов)..."
  cloudflared tunnel --url "http://localhost:$PORT" --no-autoupdate >"$LOG" 2>&1 &
  CF_PID=$!
  for _ in $(seq 1 30); do
    u="$(detect_url "$LOG")"
    [ -n "$u" ] && { URL="$u"; break; }
    if ! kill -0 "$CF_PID" 2>/dev/null; then break; fi
    sleep 1
  done
  if [ -z "$URL" ]; then
    echo "⚠️  cloudflared не отдал URL. Последние строки лога:"
    tail -n 15 "$LOG" 2>/dev/null
  fi
fi

# --- Защита от двойного ngrok (err_ngrok_3200) -----------------------------
# Бесплатный аккаунт ngrok разрешает ТОЛЬКО ОДИН одновременный агент. Если
# уже висит ngrok (например, осиротел от прошлого 'make mini' или второй
# запуск в другом терминале), второй упадёт с ERR_NGROK_3200 («account is
# limited to 1 simultaneous ngrok agent session»). Поэтому: если ngrok уже
# жив и отдаёт туннель — переиспользуем его, не запуская второй агент. Если
# висит, но не отвечает API — убиваем и стартуем заново.
if [ -z "$URL" ] && command -v ngrok >/dev/null 2>&1 && pgrep -x ngrok >/dev/null 2>&1; then
  echo "==> Найден уже запущенный ngrok — переиспользую его (без второго агента)..."
  u="$(ngrok_url)"
  if [ -n "$u" ]; then
    URL="$u"
    echo "==> 🔁 Переиспользую существующий туннель: $URL"
  else
    echo "==> Старый ngrok не отвечает API — перезапускаю один агент..."
    pkill -x ngrok 2>/dev/null || true
    sleep 2
  fi
fi

# --- Вариант 2: ngrok (самый надёжный на телефоне) -------------------------
# Требует БЕСПЛАТНЫЙ аккаунт и authtoken:
#   ngrok config add-authtoken <токен>   (https://dashboard.ngrok.com/get-started/your-authtoken)
if [ -z "$URL" ] && command -v ngrok >/dev/null 2>&1; then
  echo "==> Запуск ngrok туннеля на :$PORT (HTTPS)..."
  ngrok http "$PORT" --log=stdout >"$LOG" 2>&1 &
  NGROK_PID=$!
  for _ in $(seq 1 45); do
    u="$(detect_url "$LOG")"
    [ -z "$u" ] && u="$(ngrok_url)"
    [ -n "$u" ] && { URL="$u"; break; }
    # ngrok без токена сразу пишет об ошибке авторизации — не ждём 45с.
    # Ловим только явные сбои авторизации/запуска (не любое слово "error",
    # чтобы не обрубить ожидание раньше времени при штатных warning'ах).
    if grep -qiE "authtoken|sign up|account is limited|ERR_NGROK_3200|agent session|simultaneous|failed to start tunnel|command failed" "$LOG" 2>/dev/null; then
      if ! grep -qE "https://" "$LOG" 2>/dev/null; then break; fi
    fi
    if ! kill -0 "$NGROK_PID" 2>/dev/null; then break; fi
    sleep 1
  done
  if [ -z "$URL" ]; then
    echo "⚠️  ngrok не отдал URL. Скорее всего, не задан БЕСПЛАТНЫЙ authtoken."
    echo "     Получить токен (бесплатно): https://dashboard.ngrok.com/get-started/your-authtoken"
    echo "     Добавить его:"
    echo "        ngrok config add-authtoken <ВАШ_ТОКЕН>"
    echo "     Затем запустить 'make mini' повторно — Mini App откроется на телефоне."
    kill "$NGROK_PID" 2>/dev/null || true
    rm -f "$LOG"
    exit 1
  fi
fi

# --- Вариант 3: ставим ngrok, если его нет (самый надёжный на телефоне) ----
# Без ngrok скрипт не сможет дать стабильный HTTPS для телефона (cloudflared
# Quick-туннели дают ошибку Cloudflare на мобильных сетях — поэтому НЕ
# используем их). Устанавливаем ngrok и требуем токен.
if [ -z "$URL" ] && ! command -v ngrok >/dev/null 2>&1 && [ "${TUNNEL:-ngrok}" != "cloudflared" ]; then
  echo "⚠️  ngrok НЕ установлен. Это самый стабильный туннель для открытия"
  echo "⚠️  Mini App с телефона (cloudflared Quick-туннели дают ошибку Cloudflare)."
  if command -v brew >/dev/null 2>&1; then
    echo "==> Устанавливаю ngrok через brew (может занять пару минут — качается бинарь)..."
    if brew install ngrok/ngrok/ngrok; then
      echo "✅ ngrok установлен."
    fi
  fi
  if ! command -v ngrok >/dev/null 2>&1; then
    echo "❌ brew не смог поставить ngrok. Попробуйте вручную: https://ngrok.com/download"
    rm -f "$LOG"
    exit 1
  fi
  echo "==> Теперь добавьте БЕСПЛАТНЫЙ токен (один раз):"
  echo "     ngrok config add-authtoken <ВАШ_ТОКЕН>"
  echo "   Токен тут: https://dashboard.ngrok.com/get-started/your-authtoken"
  echo "   После этого запустите 'make mini' ещё раз — Mini App откроется на телефоне."
  rm -f "$LOG"
  exit 0
fi

# --- Вариант 4: bore.pub (без аккаунта), если установлен и доступен --------
if [ -z "$URL" ] && command -v bore >/dev/null 2>&1; then
  echo "==> Запуск bore.pub туннеля на :$PORT (HTTPS, без аккаунта)..."
  bore local "$PORT" --to bore.pub >"$LOG" 2>&1 &
  BORE_PID=$!
  for _ in $(seq 1 30); do
    u="$(detect_url "$LOG")"
    [ -n "$u" ] && { URL="$u"; break; }
    if ! kill -0 "$BORE_PID" 2>/dev/null; then break; fi
    sleep 1
  done
  if [ -z "$URL" ]; then
    echo "⚠️  bore.pub не отдал URL (возможно, недоступен из вашей сети)."
  fi
fi

# --- Если туннеля нет вообще: стартуем с LAN-IP для браузера ----------------
if [ -z "$URL" ]; then
  echo "⚠️  Не удалось поднять HTTPS-туннель. Бот стартует с LAN-IP — дашборд"
  echo "⚠️  можно открыть в БРАУЗЕРЕ телефона в той же Wi-Fi сети, но НЕ как"
  echo "⚠️  встроенный Mini App (для Mini App нужен HTTPS). Для телефона"
  echo "⚠️  рекомендуется ngrok (см. выше) или свой домен (docs/DEPLOY.md)."
  export WEBAPP_URL=""
  export DASHBOARD_URL=""
fi

# Прокидываем боту HTTPS-URL — теперь кнопка «Mini App» реально заработает.
if [ -n "$URL" ]; then
  export WEBAPP_URL="$URL/dashboard"
  export DASHBOARD_URL="$URL/dashboard"
  printf '%s/dashboard' "$URL" > "$TUNNEL_URL_FILE"
fi

# Убиваем старые инстансы бота, чтобы не боролись за токен/порт.
echo "==> Останавливаю старые инстансы бота..."
pkill -f "bin/analyzpro" 2>/dev/null || true
pkill -f "go-build/.*-d/main" 2>/dev/null || true
lsof -ti :8080 2>/dev/null | xargs kill 2>/dev/null || true
sleep 1

if [ -n "$URL" ]; then
  echo "==> 🌐 Туннель готов: $URL"
  echo "==> 🤖 WEBAPP_URL=$WEBAPP_URL  (настоящий Mini App в Telegram)"
else
  echo "==> 🤖 Бот стартует без HTTPS-туннеля (LAN-IP для браузера)."
fi
echo "==> Запуск бота... (Ctrl+C — остановит и бота, и туннель)"
"$BINARY"
