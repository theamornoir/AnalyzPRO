# Деплой AnalyzPRO

## Ошибка «Cloudflare error» при открытии Mini App с телефона

**Причина.** Telegram Mini App требует HTTPS. В dev-режиме для этого поднимается
публичный туннель. Раньше `make mini` использовал бесплатный **cloudflared
Quick-туннель** (`*.trycloudflare.com`). Cloudflare часто НЕ открывает такие
случайные поддомены с мобильных сетей и вместо страницы отдаёт экран ошибки —
это и есть «Cloudflare error» на телефоне. На десктопе в той же Wi-Fi работало,
потому что там ходило на localhost.

**Решение.** `make mini` больше НЕ использует Quick-туннели. Порядок выбора
туннеля (первый рабочий):

1. **ngrok** — самый надёжный на телефоне. Нужен бесплатный токен.
2. **Именованный Cloudflare-туннель** со своим доменом (если настроен через
   `CF_TUNNEL_URL`).
3. **bore.pub** — если установлен (без аккаунта).
4. Если ничего нет — бот стартует с LAN-IP: дашборд откроется в **браузере**
   телефона в той же Wi-Fi, но НЕ как встроенный Mini App (для него нужен HTTPS).

> Анонимные ssh-туннели (`localhost.run`, `serveo.net`) с 2024 года требуют
> зарегистрированный SSH-ключ, поэтому они не годятся «из коробки».

### ngrok (рекомендуется для телефона)

Один раз:
```bash
brew install ngrok/ngrok/ngrok                 # или скачать с ngrok.com/download
# получить бесплатный токен: https://dashboard.ngrok.com/get-started/your-authtoken
ngrok config add-authtoken <ВАШ_ТОКЕН>
```
Затем:
```bash
make mini
```
Mini App откроется на телефоне без ошибок Cloudflare.

## Прод без туннелей (финальное решение)

Cloudflare error исчезает полностью, потому что это **ваш домен с вашим TLS**.
Туннели не нужны вообще.

1. Поднимите бота на сервере с публичным доменом и валидным HTTPS
   (Caddy / nginx + Let's Encrypt, или любой PaaS с TLS).
2. Задайте переменные окружения (в `.env` рядом с бинарём):
   ```bash
   export BOT_TOKEN=<токен_бота>
   export WEBAPP_URL=https://<ваш-домен>/dashboard
   export DASHBOARD_URL=https://<ваш-домен>/dashboard
   export DB_PATH=/var/lib/analyzpro/analyzpro.db   # персистентный путь
   ```
3. Запустите бинарник: `./bin/analyzpro` (собран через `make build`).

### Чеклист перед продом
- [ ] Свой домен + HTTPS (Caddy/nginx). `WEBAPP_URL`/`DASHBOARD_URL` указывают на
      `https://<домен>/dashboard`.
- [ ] `DB_PATH` указывает на персистентный том (данные SQLite переживают рестарт).
- [ ] Роль `ai.languageModels.user` на каталог в Yandex Cloud (иначе YandexGPT —
      403).
- [ ] YooKassa подключена отдельно (в этой сборке платёж — мок, как договорено).
- [ ] `make build` проходит; `bin/analyzpro` запускается под systemd/supervisor
      с корректным `.env`.

### Именованный Cloudflare-туннель (альтернатива, если нет своего PaaS)
```bash
cloudflared login
cloudflared tunnel create analyzpro
cloudflared tunnel route dns analyzpro miniapp.<ваш-домен>.com
export CF_TUNNEL=analyzpro
export CF_TUNNEL_URL=https://miniapp.<ваш-домен>.com
make mini
```
