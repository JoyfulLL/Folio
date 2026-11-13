# Folio

A lightweight, self-hosted personal finance app. Folio records daily transactions, imports data from many sources, and lets you search, filter and visualise spending with built-in charts and custom queries. It runs comfortably on a home server, NAS or Raspberry Pi, and ships tailored desktop and mobile interfaces with PWA support.

## Features

- **Self-hosted & private** — your data stays on your own server
- **Lightweight** — SQLite, MySQL and PostgreSQL; small resource footprint
- **Cross-platform** — Windows, macOS, Linux, FreeBSD; x86, amd64 and ARM
- **Desktop + mobile UI** — PWA install, dark mode, customisable overview layouts
- **Bookkeeping** — two-level accounts and categories, picture attachments, maps, scheduled transactions, advanced filters
- **Import & export** — CSV, Excel, OFX, QFX, QIF, IIF, Camt.052/053, MT940, GnuCash, Firefly III, Beancount and more; custom column mapping, rules and scripts
- **Localisation** — 24 languages, multi-currency, multiple exchange-rate sources, multi-timezone, custom date/number formats
- **Security** — 2FA, OIDC external authentication, login rate limiting, app lock (PIN / WebAuthn)
- **Automation** — daily reports, credit-card statement and repayment reminders, Feishu / SMTP notifications, MCP and API tools

## Build from source

Requires Go, GCC (for SQLite), Node.js and npm.

```sh
# Frontend + backend build
./build.sh all

# Release package (produces folio-<version>-<arch>.tar.gz)
./build.sh package --release -o folio.tar.gz

# Docker image
./build.sh docker
```

Windows equivalents: `build.bat` and `build.ps1`.

A successful build writes the frontend into `dist/` and the backend executable to `ezbookkeeping` (`ezbookkeeping.exe` on Windows). The executable and config filenames are kept stable so existing deployments and update scripts continue to work.

## Run

```sh
./ezbookkeeping server run
```

By default Folio listens on port 8080: `http://{YOUR_HOST_ADDRESS}:8080/`.

Configuration lives in `conf/ezbookkeeping.ini`. For local development, copy it to `conf/ezbookkeeping.local.ini` and run:

```sh
go run ezbookkeeping.go server run --conf-path=conf/ezbookkeeping.local.ini
```

Every setting can be overridden with an `EBK_<SECTION>_<KEY>` environment variable.

### Docker

```sh
docker run -p8080:8080 -v /path/to/data:/ezbookkeeping/data folio:local
```

Mount a persistent volume for `data/`, `log/` and `storage/` so data survives container replacement.

## Deployment helpers

`deploy/ez-update.sh` performs an in-place, pull-based upgrade of a systemd installation. It replaces only the backend binary, `public/`, `templates/` and `LICENSE`, and never overwrites the real config, database or logs. See `deploy/ez-update.env.example` and `deploy/ezbookkeeping.service` for the expected layout.

Release tags are plain semver (`1.0.3`, no `v` prefix, so they never collide with the upstream `v*.*.*` release workflow). The tag is also the version string reported by the binary and the web UI. Release assets are named `folio-<tag>-linux-<arch>.tar.gz`.

## License

MIT. This is an independent custom build based on the MIT-licensed ezBookkeeping project; the original copyright notice and third-party license texts are preserved in [LICENSE](LICENSE) and `third-party-dependencies.json`, and are also shown in the app's About dialog.
