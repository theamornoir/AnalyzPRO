# Деплой Prisma (AnalyzPRO) на Hetzner + Caddy

Полное руководство «от нуля»: один VPS, один бот, один домен. Никаких
отдельных серверов «под профиль» не нужно — дашборд, вебхук платежей и логика
бота крутятся в **одном** Go-процессе. Caddy даёт HTTPS и проксирует трафик
на бота.

> Почему Hetzner (Финляндия/Германия): Yandex Cloud (YandexGPT) доступен из
> ЕС/США, поэтому прямое соединение работает без прокси и гео-костылей.

---

## Что получится

```
[Юзер в Telegram] ─▶ [prisma-bot.ru (HTTPS, Caddy)] ─▶ [VPS: бот :8080]
                                                                  ├─ логика бота
                                                                  ├─ /dashboard (Мой профиль)
                                                                  └─ /api/payment/webhook (YooKassa)
[YooKassa] ─▶ [prisma-bot.ru/api/payment/webhook] ─▶ [тот же бот]
```

---

## Шаг 1. Арендуй сервер на Hetzner

1. Зайди на https://www.hetzner.cloud → «Create project» → «Add server».
2. **Location:** Helsinki (FIN) или Falkenstein (DE).
3. **Image:** Ubuntu 22.04 (x86/AMD64). Берём **x86**, чтобы локальная
   кросс-компиляция была `GOARCH=amd64` (если выберешь CAX ARM — нужен
   `GOARCH=arm64`).
4. **Type:** CPX11 (2 vCPU, 4 GB RAM) — с запасом; минимум CAX11/CPX11.
5. **SSH key:** добавь свой публичный ключ (сгенерируй `ssh-keygen`, ключ —
   `~/.ssh/id_ed25519.pub`). Без пароля по SSH — безопаснее.
6. Создай сервер. Запиши **публичный IP** (вкладка «Network»).

> Фаервол Hetzner: по умолчанию открыты только 22/ICMP. Мы откроем 80/443
> ниже (через `ufw`), этого достаточно — 8080 снаружи не нужен.

---

## Шаг 2. Домен

1. Купи домен (Reg.ru / Namecheap / Timeweb), например `prisma-bot.ru`.
2. В DNS добавь **A-запись**: `@` → `<IP сервера>` и, если нужен www,
   `www` → `<IP сервера>`.
3. Подожди распространения (обычно 5–30 мин; проверь `dig +short
   prisma-bot.ru` — должен вернуть IP).

---

## Шаг 3. Подключись и подготовь сервер

```bash
ssh root@<IP_сервера>
```

На сервере:

```bash
# Обновления
apt update && apt -y upgrade

# Утилиты
apt -y install curl git ufw

# Фаервол: оставляем SSH, открываем 80/443 для Caddy/Let's Encrypt
ufw allow OpenSSH
ufw allow 80/tcp
ufw allow 443/tcp
ufw enable

# Выделенный пользователь для бота (без root)
useradd -m -s /bin/bash analyzpro
# Даём sudo временно (для установки Caddy/копирования), потом можно убрать
usermod -aG sudo analyzpro
```

Открой **второе** окно терминала и зайди под новым юзером (дальше работаем от
него, не от root):

```bash
ssh analyzpro@<IP_сервера>
```

---

## Шаг 4. Установи Caddy

```bash
sudo apt -y install debian-keyring debian-archive-keyring apt-transport-https curl
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update
sudo apt -y install caddy
```

Скопируй готовый `Caddyfile` (замени `prisma-bot.ru` на свой домен):

```bash
# На локальной машине (из корня репозитория):
scp deploy/Caddyfile analyzpro@<IP_сервера>:/tmp/Caddyfile

# На сервере:
sudo cp /tmp/Caddyfile /etc/caddy/Caddyfile
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl restart caddy
sudo systemctl enable caddy
```

Caddy сам выпустит сертификат при первом заходе на домен. Проверь позже:
`curl -I https://prisma-bot.ru/healthz` → `200 OK`.

---

## Шаг 5. Собери и положи бота

Есть два пути. **Вариант А** — собрать прямо на сервере (надёжнее, все
встроенные ассеты `webapp_files`/`templates` подтянутся через `go:embed`).
**Вариант Б** — кросс-компиляция на твоём Mac и `scp`.

### Вариант А: сборка на сервере (рекомендую)

```bash
# На сервере, под analyzpro
sudo apt -y install golang-go git
go version   # должен быть 1.21+

# Клонируй репозиторий (или закинь архивом)
git clone <твой-репозиторий> ~/src
cd ~/src

# Собери бинарь
make build
# бинарь появится в ./bin/analyzpro
```

### Вариант Б: кросс-компиляция на macOS

```bash
# На локальной машине, в корне репозитория
GOOS=linux GOARCH=amd64 go build -o bin/analyzpro ./cmd/bot
scp bin/analyzpro analyzpro@<IP_сервера>:/tmp/analyzpro
```

### Размести файлы в /opt/analyzpro

```bash
# На сервере
sudo mkdir -p /opt/analyzpro
sudo mv ~/src/bin/analyzpro /opt/analyzpro/bin/ 2>/dev/null || sudo mv /tmp/analyzpro /opt/analyzpro/bin/
sudo chmod +x /opt/analyzpro/bin/analyzpro

# .env
cp deploy/env-prod.txt /opt/analyzpro/.env   # (или закинь свой scp-ом)
# Отредактируй: nano /opt/analyzpro/.env  — заполни BOT_TOKEN, домен, YooKassa, YandexGPT

# Папки данных
sudo mkdir -p /opt/analyzpro/data /opt/analyzpro/uploads

# Права: всё принадлежит analyzpro
sudo chown -R analyzpro:analyzpro /opt/analyzpro
```

> Бинарь **включает** в себя `webapp_files/*` и `templates/*` (через
> `//go:embed`), поэтому копировать их отдельно НЕ нужно.

---

## Шаг 6. systemd-сервис

```bash
# На локальной машине:
scp deploy/analyzpro.service analyzpro@<IP_сервера>:/tmp/analyzpro.service

# На сервере:
sudo cp /tmp/analyzpro.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now analyzpro
```

Проверь, что поднялось:

```bash
sudo systemctl status analyzpro
sudo journalctl -u analyzpro -n 50 --no-pager
```

В логах должно быть:
- `[PAYMENT] YooKassa: РЕАЛЬНЫЙ режим (shopID=... задан)`
- `🌐 HTTP-сервер для Web App запускается на 127.0.0.1:8080`
- `🌐 ... запущен` / `Bot running`

Если видишь «режим СИМУЛЯЦИИ» — не заданы `YOOKASSA_SHOP_ID`/`SECRET_KEY`
(или не перечитались из `.env`).

---

## Шаг 7. Настрой YooKassa

В личном кабинете YooKassa (kassa.yandex.ru / yookassa.ru):

1. Выведи магазин из песочницы (или тестируй ключами песочницы).
2. **Уведомления → HTTP-уведомления** укажи:
   ```
   https://prisma-bot.ru/api/payment/webhook
   ```
   (должен отвечать `200 OK`; проверь `curl -I
   https://prisma-bot.ru/api/payment/webhook` — вернёт 400 без тела, это ок,
   значит роут живой).
3. Если задашь отдельный «секрет уведомлений» — продублируй его в
   `YOOKASSA_WEBHOOK_SECRET` в `.env`.

После успешной оплаты YooKassa шлёт колбэк → бот активирует Premium в БД и
записывает `tariff_id` из `metadata` платежа. Дедуп по `object.id` уже
встроен (повторный колбэк не «продлит» подписку дважды).

---

## Шаг 8. Проверь, что всё работает

```bash
# 1. HTTPS-терминация Caddy
curl -I https://prisma-bot.ru/healthz        # 200

# 2. Дашборд открывается (Web App)
curl -s https://prisma-bot.ru/dashboard | head -n 5

# 3. Вебхук «живой» (400 без тела норма)
curl -I https://prisma-bot.ru/api/payment/webhook

# 4. В Telegram: /start → пройди онбординг → «💎 Premium» → «💳 Оплатить»
#    должна открыться YooKassa, после оплаты Premium активируется.
```

---

## Обновление бота (новый релиз)

```bash
# 1. Пересобери (Вариант А на сервере):
cd ~/src && git pull && make build
sudo systemctl stop analyzpro
sudo mv ~/src/bin/analyzpro /opt/analyzpro/bin/analyzpro
sudo chown analyzpro:analyzpro /opt/analyzpro/bin/analyzpro
sudo chmod +x /opt/analyzpro/bin/analyzpro

# 2. Если менял front (webapp_files) — переподними WebAppAssetsVersion в
#    internal/bot/keyboards (сейчас "v44") и пересобери.
#    Caddy менять не нужно.

# 3. Запусти
sudo systemctl start analyzpro
sudo journalctl -u analyzpro -f
```

> При каждом обновлении `webapp_files/*` пересобираются внутрь бинаря через
> `go:embed` — просто пересобери и замени бинарь.

---

## Чек-лист перед релизом

- [ ] Домен А-запись указывает на IP; `dig` отдаёт IP.
- [ ] `ufw` открывает 22/80/443; Caddy `systemctl status caddy` — active.
- [ ] `curl -I https://<домен>/healthz` → 200.
- [ ] `.env`: `APP_ENV=production`, `WEBAPP_URL`/`DASHBOARD_URL` = `https://<домен>/dashboard`, `HTTP_ADDR=127.0.0.1:8080`.
- [ ] `YOOKASSA_SHOP_ID`/`YOOKASSA_SECRET_KEY` заданы → в логах «РЕАЛЬНЫЙ режим».
- [ ] `YANDEX_API_KEY` и `YANDEX_FOLDER_ID` заданы; роль `ai.languageModels.user` выдана.
- [ ] `systemctl status analyzpro` — active (running).
- [ ] В YooKassa URL вебхука = `https://<домен>/api/payment/webhook`.
- [ ] Удалён осиротевший `./data/premium_users.json` (если остался от старых запусков).

---

## Что НЕ нужно

- ❌ Отдельный сервер «под дашборд» — его раздаёт сам бот.
- ❌ Покупать TLS-сертификат — Caddy делает Let's Encrypt бесплатно.
- ❅ Прокси для YandexGPT не требуется — запросы ходят напрямую в Yandex Cloud.
- ❅ Настраивать Telegram Webhook — бот использует long-polling (сам ходит в
      api.telegram.org). Открывать порт для Telegram не нужно.
