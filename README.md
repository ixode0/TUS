<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="tus_logo.png">
    <source media="(prefers-color-scheme: light)" srcset="tus_logo_dark.png">
    <img src="tus_logo.png" alt="TUS logo" width="320" />
  </picture>
</p>

<p align="center">
  <b>TUS</b> — <i>Telegram Username Sniper</i>
</p>

# TUS

🇷🇺 Молниеносный снайпер Telegram-юзернеймов на **Go**. Автоматически мониторит и забирает юзернеймы, как только они становятся доступны.

🇬🇧 Lightning-fast Telegram username sniper in **Go**. Automatically monitors and claims usernames the moment they become available.

Обновлённая версия / Updated version: **Go 1.25** + **github.com/gotd/td v0.161.0** (последний MTProto-слой Telegram / latest Telegram MTProto layer).

---

## 🤖 Промт для ИИ / AI Prompt

### 🇷🇺 Русский
Забудьте про ручную правку `config.json`. Если вы пользуетесь ИИ-помощником — скопируйте этот промт, и он настроит всё сам:

```
Вот репозиторий снайпера Telegram-юзернеймов: TUS (Go).

1. Склонируй репо из корня, выполни go mod tidy и собери: go build -o sniper ./cmd/app (Go 1.25+, модуль app, бинарь окажется рядом с config.json).
2. Запроси у пользователя по очереди:
   - API ID и API Hash (он их получит на https://my.telegram.org → API development tools)
   - номер телефона в международном формате (+..., с плюсом — валидация строгая)
   - список юзернеймов для мониторинга (массив usernames, без @)
   - способ клейма: channel (создать публичный канал с title = username и затем установить юзернейм) или user (account.updateUsername на аккаунт)
   - паузу между проверками в мс (sleep_between_check, рекомендуется 100, <50 — уже риск FloodWait)
3. Заполни config.json этими данными (лежит в корне репо; можно переопределить путь через env CONFIG_PATH).
4. Запусти ./sniper рядом с config.json.
5. При первом запуске программа запросит код авторизации — попроси его у пользователя и передай программе через stdin. Для автоматики: можно заранее задать env TG_CODE или положить код в файл /tmp/tg_code.txt (файл удалится после чтения). Если у аккаунта включена 2FA — следующим шагом запросит пароль (Enter your password:).
6. Сессия сохранится в session_DO_NOT_SHARE.json рядом с бинарём (FileStorage) — добавь его в .gitignore и никому не показывай.
7. Если снайпер не смог забрать юзернейм сразу — он будет ретраить каждые 1.5с бесконечно (отдельная горутина на каждый юзернейм), с корректной обработкой FloodWait (ждёт указанное время) и остановкой при USERNAME_OCCUPIED / USERNAME_NOT_AVAILABLE. Отчитайся пользователю, когда юзернейм будет успешно захвачен (лог [Successfully claimed: ...]).
```

Этот промт особенно удобен, если у пользователя нет окружения Go — ИИ сделает сборку и запуск за него.

### 🇬🇧 English
Forget manual `config.json` editing. If you use an AI assistant — copy this prompt and it will set everything up:

```
Here's the Telegram username sniper repository: TUS (Go).

1. Clone the repo from its root, run go mod tidy and build: go build -o sniper ./cmd/app (Go 1.25+, module app, binary lands next to config.json).
2. Ask the user step-by-step for:
   - API ID and API Hash (from https://my.telegram.org → API development tools)
   - phone number in international format (+..., with plus — strict validation)
   - list of usernames to monitor (usernames array, without @)
   - claim method: channel (create public channel with title = username then set username) or user (account.updateUsername on the account)
   - delay between checks in ms (sleep_between_check, recommended 100, <50 risks FloodWait)
3. Fill config.json with this data (located at repo root; path can be overridden via env CONFIG_PATH).
4. Run ./sniper next to config.json.
5. On first launch the app will request an auth code — ask the user for it and pass it via stdin. For automation: pre-set env TG_CODE or put the code into /tmp/tg_code.txt (file is deleted after reading). If the account has 2FA enabled — next it will ask for password (Enter your password:).
6. Session will be saved to session_DO_NOT_SHARE.json next to the binary (FileStorage) — add it to .gitignore and never share it.
7. If the sniper fails to claim immediately — it will retry every 1.5s forever (separate goroutine per username), with proper FloodWait handling (waits the required duration) and stopping on USERNAME_OCCUPIED / USERNAME_NOT_AVAILABLE. Notify the user when the username is successfully claimed (log [Successfully claimed: ...]).
```

This prompt is especially handy if the user has no Go environment — the AI will handle build and run.

---

## ⚙️ Как это работает / How It Works

### 🇷🇺 Русский
1. Сервис проверяет доступность юзернейма через [fragment.com](https://fragment.com/).
2. Как только юзернейм становится `Available`, снайпер через [Telegram API](https://core.telegram.org/api) либо **создаёт новый канал**, либо **назначает юзернейм на аккаунт** — зависит от настроек.
3. При неудаче повторяет попытку каждые **1.5 секунды** до успеха (с учётом `FloodWait`).

### 🇬🇧 English
1. The service checks username availability via [fragment.com](https://fragment.com/).
2. As soon as the username becomes `Available`, the sniper via [Telegram API](https://core.telegram.org/api) either **creates a new channel** or **assigns the username to the account** — depending on settings.
3. On failure it retries every **1.5 seconds** until success (respecting `FloodWait`).

---

## ✨ Возможности / Features

### 🇷🇺 Русский
- [x] Гибкий способ клейма: `channel` (канал) или `user` (аккаунт)
- [x] Несколько юзернеймов в списке
- [x] Авто-удаление уже занятых юзернеймов из мониторинга
- [x] Бесконечный ретрай каждые 1.5с + учёт `FloodWait`
- [x] Интерактивная авторизация Telegram
- [x] Постоянная сессия (`session_DO_NOT_SHARE.json`)
- [x] Публичный канал создаётся автоматически с `title = username`
- [ ] Прокси
- [ ] Несколько аккаунтов

### 🇬🇧 English
- [x] Flexible claim method: `channel` or `user`
- [x] Multiple usernames in watchlist
- [x] Auto-removal of already taken usernames from monitoring
- [x] Infinite retry every 1.5s + `FloodWait` handling
- [x] Interactive Telegram authorization
- [x] Persistent session (`session_DO_NOT_SHARE.json`)
- [x] Public channel is created automatically with `title = username`
- [ ] Proxy
- [ ] Multiple accounts

---

## ❓ FAQ / Частые вопросы

### 🇷🇺 Русский
**Q: Можно ли снайпить юзернеймы на аукционе fragment?**
**A:** Нет. Юзернеймы на аукционе автосвопятся и никогда не становятся доступны для клейма.

**Q: Можно ли использовать bot token?**
**A:** Нет. Боты не имеют доступа к `channels.createChannel` и `account.updateUsername`, которые требуются снайперу. Нужен обычный аккаунт (user).

**Q: Почему лучше клеймить в канал?**
**A:** У каналов меньше рейт-лимитов, а юзернейм канала всё равно можно продать на fragment. Настраивается через `claim_to`.

**Q: Сколько юзернеймов мониторить?**
**A:** Не рекомендуется больше 5 за раз — при большом количестве есть шанс пропустить момент освобождения. Лучше меньше юзов + терпение.

**Q: Как находить юзернеймы под снайпинг?**
**A:**
1. Онлайн-площадки продажи Telegram-юзернеймов.
2. Неактивные аккаунты («last seen a long time ago») — такие могут быть удалены в ближайшие 1-12 месяцев.

### 🇬🇧 English
**Q: Can I snipe usernames that are on fragment auction?**
**A:** No. Auction usernames are auto-swapped and never become available to claim.

**Q: Can I use a bot token?**
**A:** No. Bots don't have access to `channels.createChannel` and `account.updateUsername` required by the sniper. You need a regular user account.

**Q: Why is claiming to a channel better?**
**A:** Channels have fewer rate limits, and a channel username can still be sold on fragment. Configured via `claim_to`.

**Q: How many usernames should I monitor?**
**A:** Not recommended more than 5 at once — with many usernames you may miss the release moment. Fewer targets + patience is better.

**Q: How to find usernames to snipe?**
**A:**
1. Online Telegram username marketplaces.
2. Inactive accounts ("last seen a long time ago") — such accounts may be deleted in the next 1-12 months.

---

## 🔧 Конфигурация / Configuration

### 🇷🇺 Русский
Скопируйте `config.json` и заполните:

```json
{
    "telegram": {
        "phone_number": "+1234567890",
        "api_id": 12345,
        "api_hash": "abcdef1234567890"
    },
    "claim_to": "channel",
    "sleep_between_check": 100,
    "usernames": [
        "your_username"
    ]
}
```

| Поле | Описание |
|------|----------|
| `telegram.phone_number` | Номер аккаунта в международном формате (`+1234567890`) |
| `telegram.api_id` / `telegram.api_hash` | Получить на https://my.telegram.org → *API development tools* |
| `claim_to` | Способ клейма: `channel` или `user` |
| `sleep_between_check` | Пауза между проверками, мс. Менее `100` может вызвать рейт-лимиты |
| `usernames` | Список юзернеймов для мониторинга (например `["dead", "devious"]`) |

### 🇬🇧 English
Copy `config.json` and fill it:

```json
{
    "telegram": {
        "phone_number": "+1234567890",
        "api_id": 12345,
        "api_hash": "abcdef1234567890"
    },
    "claim_to": "channel",
    "sleep_between_check": 100,
    "usernames": [
        "your_username"
    ]
}
```

| Field | Description |
|-------|-------------|
| `telegram.phone_number` | Account phone in international format (`+1234567890`) |
| `telegram.api_id` / `telegram.api_hash` | Get at https://my.telegram.org → *API development tools* |
| `claim_to` | Claim method: `channel` or `user` |
| `sleep_between_check` | Delay between checks, ms. Below `100` may trigger rate limits |
| `usernames` | List of usernames to watch (e.g. `["dead", "devious"]`) |

---

## 🚀 Запуск / Running

### 🇷🇺 Русский
```bash
# Требуется Go 1.25+
go mod tidy

# из папки cmd/app
cd cmd/app
go run .
```

Или собрать бинарь и запустить рядом с `config.json`:
```bash
go build -o sniper ./cmd/app
./sniper
```

При первом запуске будет запрошен код авторизации из Telegram — он придёт в приложение аккаунта. Если включена 2FA — далее запросит пароль (`Enter your password:`). После этого сессия сохранится в `session_DO_NOT_SHARE.json` (не делитесь и не коммитьте этот файл).

> Не-интерактивный запуск: код можно подложить в `TG_CODE` (env) или файл `/tmp/tg_code.txt` (удалится после чтения). Пароль 2FA при необходимости — через stdin (скрытый ввод).

### 🇬🇧 English
```bash
# Requires Go 1.25+
go mod tidy

# from cmd/app folder
cd cmd/app
go run .
```

Or build the binary and run next to `config.json`:
```bash
go build -o sniper ./cmd/app
./sniper
```

On first launch you'll be asked for the Telegram auth code — it will arrive in your account's app. If 2FA is enabled — it will then ask for password (`Enter your password:`). Afterwards the session is saved to `session_DO_NOT_SHARE.json` (do not share or commit this file).

> Non-interactive run: you can provide the code via `TG_CODE` env or `/tmp/tg_code.txt` file (deleted after reading). 2FA password if needed — via stdin (hidden input).

---

## 📜 Происхождение / Credits

### 🇷🇺 Русский
Частично походит из оригинального проекта: [qg5/telegram-username-sniper](https://github.com/qg5/telegram-username-sniper) (MIT).

### 🇬🇧 English
Partially based on the original project: [qg5/telegram-username-sniper](https://github.com/qg5/telegram-username-sniper) (MIT).

Лицензия / License: [MIT](LICENSE)
