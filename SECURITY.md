# Security Policy

## Supported Versions

| Version | Supported |
| ------- | --------- |
| v1.x (main) | ✅ |

## Reporting a Vulnerability

Open a GitHub Security Advisory or issue with label `security`.
Do **not** post session files, `config.json`, api_hash or phone numbers in public issues.

## Secrets hygiene

- `config.json` is git-ignored — copy from `config.example.json`.
- Never commit `session_DO_NOT_SHARE.json`, `*.session`, `TG_CODE` or `/tmp/tg_code.txt` contents.
- If a session or api_hash leaked: revoke the session in Telegram
  (Settings → Devices → Terminate), regenerate API credentials at
  https://my.telegram.org, delete the file and rotate secrets.
- The code warns when the code-file is group/world-readable; keep it `chmod 600`.
