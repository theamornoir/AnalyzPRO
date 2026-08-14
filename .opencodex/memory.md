# AnalyzPRO — устойчивые факты

## Общение
- С этим пользователем общаться на русском языке.

## Проект (AnalyzPRO Telegram-бот, Go)
- Локальное тестирование Mini App на телефоне: `make mini` поднимает бота + HTTPS-туннель и прокидывает `WEBAPP_URL=<туннель>/dashboard`. Menu Button настроен открывать дашборд как настоящий Telegram Mini App.
- **Остановка из ЛЮБОГО терминала:** `make mini-stop` — убивает запущенный `make mini` по lock-PID (`/tmp/analyzpro-mini.lock/pid`), туннель (ngrok/cloudflared), бота, освобождает `:8080` и снимает lock. Реализовано через `scripts/mini-stop.sh` (Makefile-цель `mini-stop`). Это решает проблему «Ctrl+C не работает, потому что `make mini` крутится в чужом/невидимом терминале».
- Туннель по умолчанию — ngrok (бесплатный план = лимит **1 агент** → `err_ngrok_3200`). Повторяющаяся проблема: пользователь запускал `make mini` в нескольких терминалах → два ngrok-агента → ошибка.
- `scripts/run-miniapp.sh` теперь:
  - (1) **singleton через `mkdir`-lock (`/tmp/analyzpro-mini.lock`) с PID-файлом** (`$LOCKDIR/pid`). Второй `make mini` не стартует, а пишет «уже запущен» и выходит. Ключевое: lock **PID-зависимый** — если терминал был убит/закрыт и процесс-владелец мёртв, lock считается устаревшим и `make mini` его снимает и стартует сам (раньше висевший lock блокировал все последующие запуски с «Уже запущен»).
  - (2) опция `TUNNEL=cloudflared make mini` — быстрый туннель Cloudflare без лимита агентов (на части моб. сетей бывает ошибка Cloudflare → вернуться на ngrok).
  - (3) `LOG` больше не через `mktemp /tmp/analyzpro-tunnel.XXXXXX.log` (на macOS такой шаблон трактуется буквально → «mktemp: mkstemp failed … File exists»). Теперь `LOG="/tmp/analyzpro-tunnel-$$.log"` (PID-суффикс) — коллизий нет.
  - cleanup (trap EXIT/INT/TERM) удаляет pid-файл и lock-каталог.
- `flock` на macOS НЕТ — синглтон сделан через атомарный `mkdir` + PID-проверка.
- Экран Telegram «you are about to visit <domain>» — НЕ ошибка, штатное подтверждение перед любым Mini App; нажимать «Открыть».

## Деплой / прод
- Для прод нужен свой домен с HTTPS + `WEBAPP_URL=https://<домен>/dashboard`; туннели (ngrok/cloudflared) тогда не нужны. Возможная опция — `Caddyfile`/`docs/DEPLOY.md`.
- В репо есть `analyzpro.service` (systemd unit) — единичный инстанс бота, блокировка через flock `/tmp/analyzpro.lock` (для Linux-сервера, не для macOS-локала).
