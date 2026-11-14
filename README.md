# Folio

[English](README.md) | [简体中文](README.zh-CN.md)

A lightweight, self-hosted personal finance app. Folio records daily transactions, imports data from many sources, and lets you search, filter and visualise spending with built-in charts and custom queries. It runs comfortably on a home server, NAS or Raspberry Pi, and ships tailored desktop and mobile interfaces with PWA support.

## Highlights

Folio is an independent custom build forked from [ezBookkeeping](https://github.com/mayswind/ezbookkeeping) at commit `a92a1081` (2026-11-10). The highlights below are what it adds; general features follow.

### Credit cards

- **Shared credit-limit pools** — one credit-card account acts as a shared-limit container and its sub-accounts are the individual cards. Used amounts across every card in the pool are aggregated to compute the remaining credit, with cross-currency conversion derived from the per-currency pool limits. The *Physical Card* switch marks whether an account is a real card or only a limit container; the per-card bank limit is stored for reference and does not drive the calculation. Hidden-balance mode is respected.
- **Statement-date transaction toggle** — choose whether transactions made *on* the statement date belong to the current billing cycle or the next one. The choice propagates through billing-cycle boundaries, date-range filters and statistics, on both desktop and mobile.
- **Card profiles** — bank name, card name, last four digits, cardholder name, expiration month/year, and an uploaded card picture stored as a separate file (with an endpoint that purges unused pictures). Extra fields are tucked into an expandable *Advanced Card Details* section.
- **Repayment-date rules** — either "N days after the statement date" or a fixed day each month.
- **Credit-card email notifications** — statement-date reminders, a statement email containing every transaction of the billing cycle that just ended, and repayment reminders with a configurable number of days in advance. Each user picks a notification time in their own timezone and a check interval from 15 minutes to 12 hours; a 15-minute cron job plans deliveries and catches up on missed ones.

### Automation

- **Daily transaction reports to Feishu** — push a daily income/expense summary to a Feishu group through a custom bot webhook. Per-user settings cover the webhook, signing secret (encrypted with the server secret key before storage), custom keyword, send time, and a "send test message" action. A 15-minute cron job delivers at or after the configured local time (up to 15 minutes late) and records a delivery log.

### Interface

- **Uniform account icon badges** — account cards use a single 40×40 circular badge with a 24px `contain`-fitted icon and theme-aware tint tokens, replacing the large translucent card watermarks. Light and dark themes share the same design tokens.
- **Insights Explorer visibility preference** — show or hide the Insights Explorer entry in the desktop navbar (upstream always shows it).
- **Folio branding** — app name, PWA icons and export filename prefixes use *Folio*; the About screens credit "Built on ezBookkeeping".

### Build & deployment

- Pull-based in-place upgrade script (`deploy/ez-update.sh`), a systemd unit and MySQL bootstrap SQL.
- CI packaging workflow; release assets named `folio-<version>-<arch>.tar.gz` with plain semver tags that do not collide with upstream `v*.*.*` tags.
- Build scripts, Dockerfile and PWA tooling adjusted for the renamed release artifacts.

### Upstream baseline

The fork is pinned to upstream `a92a1081` (2026-11-10). Later upstream work — MT940 ING `:86:` field parsing, QIF padded-date and Beancount import fixes, MCP tool annotations, dropdown/textarea polish and translation updates — has not been merged yet. This is a version gap, not an intentional feature removal.

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
