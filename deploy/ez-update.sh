#!/usr/bin/env bash
# ez-update.sh - in-place, pull-based deploy for the custom ezBookkeeping build.
#
# Works on any Linux (Ubuntu, Debian, RHEL-family / Oracle Linux 10, ...).
# On RHEL-family systems SELinux is enforcing by default, so replacements keep
# the destination's inode/directory label instead of recreating it.
#
# Designed for the EXISTING layout (no path changes, nginx does not need touching):
#   /opt/ez/ezbookkeeping            binary
#   /opt/ez/public                  frontend (nginx root/proxy target stays the same)
#   /opt/ez/conf/ezbookkeeping.ini  REAL config (secret_key, DB password) - NEVER touched
#   /opt/ez/{data,storage,log}      user data - NEVER touched
#   /opt/ez-backups/                backups created by this script
#
# Usage:
#   sudo ez-update.sh --check                 # dry run: show every resolved path, change nothing
#   sudo ez-update.sh                        # deploy RELEASE_TAG from /etc/ez-update.env
#   sudo ez-update.sh credit-1.0.1           # deploy one specific Release tag
#   sudo ez-update.sh latest                 # deploy the newest Release
#   sudo ez-update.sh --rollback             # restore the most recent app backup
#
# What an update does:
#   1. download the arch-matched tar.gz from the GitHub Release
#   2. stop the service, back up the database
#   3. back up the 4 build artifacts into /opt/ez-backups/app-<timestamp>
#   4. replace ONLY ezbookkeeping / public / templates / LICENSE in /opt/ez
#      (conf/, data/, storage/, log/ and every other file are untouched)
#   5. start the service, probe /healthz.json, roll back automatically on failure
#   6. keep the last KEEP_BACKUPS backups
set -Eeuo pipefail
IFS=$'\n\t'

CONFIG_FILE="${EZ_UPDATE_CONFIG:-/etc/ez-update.env}"

# ---- defaults, all overridable in $CONFIG_FILE -----------------------------
INSTALL_DIR="/opt/ez"
BACKUP_DIR="/opt/ez-backups"
CONF_FILE=""                 # empty => $INSTALL_DIR/conf/ezbookkeeping.ini
SQLITE_DB_PATH=""            # empty => $INSTALL_DIR/data/ezbookkeeping.db
SERVICE_NAME="ezbookkeeping"
SERVICE_USER="ezbookkeeping"
SERVICE_GROUP="ezbookkeeping"
GITHUB_REPO=""
GITHUB_TOKEN=""
RELEASE_TAG=""
HEALTH_URL=""                # empty => derive from [server] in the ini
HEALTH_RETRIES="30"
HEALTH_INTERVAL="2"
KEEP_BACKUPS="3"
DB_BACKUP_CMD=""

# Only these four build artifacts are ever replaced.
ITEMS=(ezbookkeeping public templates LICENSE)
# These are explicitly protected - a bug must never delete them.
PROTECTED=(conf data storage log)

log() { printf '[ez-update] %s\n' "$*"; }
die() { printf '[ez-update] ERROR: %s\n' "$*" >&2; exit 1; }

usage() {
    sed -n '2,30p' "$0" | sed 's/^# \{0,1\}//'
    exit 0
}

TMP_DIR=""
STAGE=""
APP_BACKUP=""
SERVICE_STOPPED=0

cleanup() {
    local rc=$?
    if [ -n "$STAGE" ] && [ -d "$STAGE" ]; then rm -rf -- "$STAGE"; fi
    if [ -n "$TMP_DIR" ] && [ -d "$TMP_DIR" ]; then rm -rf -- "$TMP_DIR"; fi
    # Never leave the service down because an update aborted (e.g. mysqldump
    # failed, package was corrupt, Ctrl-C). Only kicks in on a non-zero exit
    # while we still own the stop; successful runs reset SERVICE_STOPPED=0.
    if [ "$SERVICE_STOPPED" = "1" ] && [ "$rc" -ne 0 ]; then
        systemctl start "$SERVICE_NAME" >/dev/null 2>&1 || true
        printf '[ez-update] ERROR: update aborted (rc=%s); service restarted\n' "$rc" >&2
    fi
    return 0
}
trap cleanup EXIT INT TERM

# ---- mode ------------------------------------------------------------------
MODE="update"
case "${1:-}" in
    --check)    MODE="check";    shift ;;
    --rollback) MODE="rollback"; shift ;;
    --help|-h)  usage ;;
esac

# ---- load config -----------------------------------------------------------
[ "$(id -u)" -eq 0 ] || die "must run as root: sudo $0 <tag>"
[ -f "$CONFIG_FILE" ] || die "missing config $CONFIG_FILE (see deploy/ez-update.env.example)"
# shellcheck disable=SC1090
set -a
. "$CONFIG_FILE"
set +a

[ -n "$GITHUB_REPO" ] || die "GITHUB_REPO must be set in $CONFIG_FILE (owner/repo)"
[ -n "$CONF_FILE" ] || CONF_FILE="$INSTALL_DIR/conf/ezbookkeeping.ini"
[ -n "$SQLITE_DB_PATH" ] || SQLITE_DB_PATH="$INSTALL_DIR/data/ezbookkeeping.db"

for cmd in curl tar systemctl find awk date mktemp; do
    command -v "$cmd" >/dev/null 2>&1 || die "required command not found: $cmd"
done

if command -v python3 >/dev/null 2>&1; then
    JSON_TOOL="python3"
elif command -v jq >/dev/null 2>&1; then
    JSON_TOOL="jq"
else
    die "python3 or jq is required to parse the GitHub API response (Oracle Linux: sudo dnf install -y python3)"
fi

# Read one key from one section of the ini without a Python dependency.
ini_get() {
    awk -v sec="$1" -v key="$2" '
        /^[[:space:]]*\[/ { insec = ($0 ~ "\\[" sec "\\]"); next }
        insec && $0 ~ "^[[:space:]]*" key "[[:space:]]*=" {
            sub(/^[^=]*=[[:space:]]*/, "")
            sub(/[[:space:]]+#.*$/, "")
            sub(/[[:space:]]+$/, "")
            print
            exit
        }
    ' "$CONF_FILE"
}

detect_health_url() {
    [ -f "$CONF_FILE" ] || return 0
    local proto addr port
    proto="$(ini_get server protocol)"; [ -n "$proto" ] || proto="http"
    addr="$(ini_get server http_addr)"; [ -n "$addr" ] || addr="127.0.0.1"
    port="$(ini_get server http_port)"; [ -n "$port" ] || return 0
    case "$proto" in http|https) ;; *) return 0 ;; esac
    case "$addr" in 0.0.0.0|::|"") addr="127.0.0.1" ;; esac
    printf '%s://%s:%s/healthz.json\n' "$proto" "$addr" "$port"
}

if [ -z "$HEALTH_URL" ]; then
    HEALTH_URL="$(detect_health_url)"
    [ -n "$HEALTH_URL" ] || HEALTH_URL="http://127.0.0.1:6060/healthz.json"
fi

case "$(uname -m)" in
    x86_64|amd64)  ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) die "unsupported CPU architecture $(uname -m); build the matching package first" ;;
esac

# ---- shared helpers --------------------------------------------------------
api_get() {
    local path="$1"
    local -a args=(-fsSL --connect-timeout 15 --max-time 60
                   -H "Accept: application/vnd.github+json"
                   -H "X-GitHub-Api-Version: 2022-11-28")
    if [ -n "$GITHUB_TOKEN" ]; then
        args+=(-H "Authorization: Bearer $GITHUB_TOKEN")
    fi
    curl "${args[@]}" "https://api.github.com/repos/$GITHUB_REPO$path"
}

json_tag() {
    if [ "$JSON_TOOL" = "python3" ]; then
        python3 -c 'import json,sys; print(json.load(sys.stdin).get("tag_name",""))'
    else
        jq -r '.tag_name // ""'
    fi
}

json_asset_url() {
    if [ "$JSON_TOOL" = "python3" ]; then
        ASSET_NAME="$ASSET_NAME" python3 -c 'import json,sys,os
n=os.environ["ASSET_NAME"]
d=json.load(sys.stdin)
print(next((a["url"] for a in d.get("assets",[]) if a["name"]==n), ""))'
    else
        jq -r --arg n "$ASSET_NAME" '.assets[]? | select(.name==$n) | .url' | sed -n '1p'
    fi
}

assert_safe_item() {
    local item="$1" p
    for p in "${PROTECTED[@]}"; do
        if [ "$item" = "$p" ]; then
            die "refusing to touch protected path: $item"
        fi
    done
    return 0
}

backup_items() {
    local dest="$1" item
    mkdir -p "$dest"
    for item in "${ITEMS[@]}"; do
        assert_safe_item "$item"
        if [ -e "$INSTALL_DIR/$item" ]; then
            # -a also preserves the SELinux context: the backup doubles as the
            # reference label used by fix_selinux_contexts() below.
            cp -a "$INSTALL_DIR/$item" "$dest/"
        fi
    done
}

# Replace one build artifact WITHOUT recreating its inode / directory.
# RHEL-family distros (Oracle Linux 10, RHEL, Rocky, Alma) run SELinux in
# enforcing mode by default. "rm -rf" + "cp -a" would import the staging
# directory's label into /opt/ez and systemd could fail with 203/EXEC.
# Keeping the destination makes new files inherit exactly the label that
# already works with systemd and nginx today.
replace_item() {
    local src="$1" item="$2" dest="$INSTALL_DIR/$item"
    assert_safe_item "$item"
    [ -e "$src/$item" ] || return 0
    if [ -d "$src/$item" ]; then
        mkdir -p "$dest"
        find "$dest" -mindepth 1 -delete
        # No -a here: --preserve=context would copy the staging label instead.
        cp -R --no-dereference "$src/$item/." "$dest/"
    else
        # Overwriting an existing regular file keeps its inode and its label.
        cp -f "$src/$item" "$dest"
    fi
    chown -R "$SERVICE_USER:$SERVICE_GROUP" "$dest"
}

# Belt and braces for enforcing SELinux: force the top-level label of each
# deployed item to match the backed-up original. No-op when SELinux is off.
fix_selinux_contexts() {
    command -v getenforce >/dev/null 2>&1 || return 0
    [ "$(getenforce 2>/dev/null || true)" = "Enforcing" ] || return 0
    command -v chcon >/dev/null 2>&1 || return 0
    [ -n "$APP_BACKUP" ] && [ -d "$APP_BACKUP" ] || return 0
    local item
    for item in "${ITEMS[@]}"; do
        if [ -e "$APP_BACKUP/$item" ] && [ -e "$INSTALL_DIR/$item" ]; then
            chcon --reference="$APP_BACKUP/$item" "$INSTALL_DIR/$item" >/dev/null 2>&1 || true
        fi
    done
}

install_items() {
    local src="$1" item
    for item in "${ITEMS[@]}"; do
        replace_item "$src" "$item"
    done
    chmod 755 "$INSTALL_DIR/ezbookkeeping"
}

restore_items() {
    local src="$1" item dest
    for item in "${ITEMS[@]}"; do
        assert_safe_item "$item"
        dest="$INSTALL_DIR/$item"
        if [ ! -e "$src/$item" ]; then
            rm -rf -- "$dest"
            continue
        fi
        replace_item "$src" "$item"
    done
    chmod 755 "$INSTALL_DIR/ezbookkeeping"
}

latest_app_backup() {
    [ -d "$BACKUP_DIR" ] || return 0
    local -a rows=()
    # Avoid "head" here: with pipefail it can SIGPIPE sort and abort the script.
    mapfile -t rows < <(find "$BACKUP_DIR" -mindepth 1 -maxdepth 1 -type d -name 'app-*' \
                        -printf '%T@ %p\n' | sort -rn)
    if (( ${#rows[@]} > 0 )); then
        printf '%s' "${rows[0]#* }"
    fi
}

probe() {
    local i body=""
    for ((i = 1; i <= HEALTH_RETRIES; i++)); do
        body="$(curl -fsSk --noproxy '*' --max-time 3 "$HEALTH_URL" 2>/dev/null || true)"
        case "$body" in
            *'"status":"ok"'*) printf '%s' "$body"; return 0 ;;
        esac
        sleep "$HEALTH_INTERVAL"
    done
    printf '%s' "$body"
    return 1
}

prune() {
    local dir="$1" pattern="$2" keep="$3"
    [ -d "$dir" ] || return 0
    local -a items=()
    mapfile -t items < <(find "$dir" -mindepth 1 -maxdepth 1 -name "$pattern" \
                         -printf '%T@ %p\n' | sort -n | cut -d' ' -f2-)
    local total=${#items[@]} i
    if (( total <= keep )); then return 0; fi
    for ((i = 0; i < total - keep; i++)); do
        rm -rf -- "${items[i]}"
        log "pruned ${items[i]}"
    done
}

rollback() {
    log "start/health check failed - rolling back from $APP_BACKUP"
    SERVICE_STOPPED=1
    systemctl stop "$SERVICE_NAME" || true
    restore_items "$APP_BACKUP"
    fix_selinux_contexts
    if systemctl start "$SERVICE_NAME"; then
        SERVICE_STOPPED=0
    fi
    log "rolled back. inspect: journalctl -u $SERVICE_NAME -n 50 --no-pager"
    exit 1
}

# ---- --check : read-only inspection ---------------------------------------
if [ "$MODE" = "check" ]; then
    log "MODE: check (nothing will be changed)"
    log "install dir      : $INSTALL_DIR  ($([ -d "$INSTALL_DIR" ] && echo exists || echo MISSING))"
    log "config file      : $CONF_FILE  ($([ -f "$CONF_FILE" ] && echo exists || echo MISSING))"
    log "binary           : $INSTALL_DIR/ezbookkeeping  ($([ -f "$INSTALL_DIR/ezbookkeeping" ] && echo exists || echo MISSING))"
    log "systemd unit     : $(systemctl show -p FragmentPath --value "$SERVICE_NAME" 2>/dev/null || echo 'not found')"
    log "WorkingDirectory : $(systemctl show -p WorkingDirectory --value "$SERVICE_NAME" 2>/dev/null || echo '?')"
    log "service state    : $(systemctl is-active "$SERVICE_NAME" 2>/dev/null || echo '?')"
    log "health url       : $HEALTH_URL"
    log "current health   : $(curl -fsSk --noproxy '*' --max-time 3 "$HEALTH_URL" 2>/dev/null || echo 'not reachable')"
    log "database type    : $(ini_get database type 2>/dev/null || echo '?')"
    log "db backup cmd    : ${DB_BACKUP_CMD:-<EMPTY - will only copy sqlite db if present>}"
    log "sqlite db        : $SQLITE_DB_PATH  ($([ -f "$SQLITE_DB_PATH" ] && echo exists || echo 'not used'))"
    log "repository       : $GITHUB_REPO   token: $([ -n "$GITHUB_TOKEN" ] && echo set || echo none)"
    log "cpu / package    : $(uname -m) -> linux-$ARCH"
    OS_LABEL="$(awk -F= '/^PRETTY_NAME=/ { gsub(/"/, "", $2); print $2; exit }' /etc/os-release 2>/dev/null || true)"
    SELINUX_MODE="n/a"
    if command -v getenforce >/dev/null 2>&1; then
        SELINUX_MODE="$(getenforce 2>/dev/null || echo '?')"
    fi
    BIN_LABEL="n/a (SELinux not enforcing)"
    if [ "$SELINUX_MODE" = "Enforcing" ]; then
        BIN_LABEL="$(stat -c '%C' "$INSTALL_DIR/ezbookkeeping" 2>/dev/null || echo '?')"
    fi
    log "os / selinux     : ${OS_LABEL:-unknown} / $SELINUX_MODE"
    log "binary label     : $BIN_LABEL"
    log "json parser      : $JSON_TOOL"
    log "requested tag    : ${1:-${RELEASE_TAG:-latest}}"
    log "WILL REPLACE     : $(printf '%s ' "${ITEMS[@]}")"
    log "NEVER TOUCH      : $(printf '%s ' "${PROTECTED[@]}") (and any other file in $INSTALL_DIR)"
    log "backups go to    : $BACKUP_DIR"
    exit 0
fi

# ---- --rollback : restore last app backup ---------------------------------
if [ "$MODE" = "rollback" ]; then
    APP_BACKUP="$(latest_app_backup)"
    [ -n "$APP_BACKUP" ] || die "no app-* backup found in $BACKUP_DIR"
    log "restoring $APP_BACKUP"
    SERVICE_STOPPED=1
    systemctl stop "$SERVICE_NAME" || true
    restore_items "$APP_BACKUP"
    fix_selinux_contexts
    systemctl daemon-reload
    systemctl start "$SERVICE_NAME" || die "service did not start after rollback"
    SERVICE_STOPPED=0
    if BODY="$(probe)"; then
        log "OK, rolled back and healthy: $BODY"
    else
        log "WARNING: service started but health probe failed: $HEALTH_URL"
        exit 1
    fi
    exit 0
fi

# ---- resolve tag and asset -------------------------------------------------
TAG="${1:-${RELEASE_TAG:-latest}}"
[ -n "$TAG" ] || TAG="latest"
if [ "$TAG" = "latest" ]; then
    TAG="$(api_get "/releases/latest" | json_tag)"
    [ -n "$TAG" ] || die "cannot resolve the latest release (check repo / token / network)"
    log "latest release: $TAG"
fi

ASSET_NAME="ezbookkeeping-${TAG}-linux-${ARCH}.tar.gz"
log "target: $GITHUB_REPO / $TAG / $ASSET_NAME"

TMP_DIR="$(mktemp -d)"
PKG="$TMP_DIR/$ASSET_NAME"

if [ -n "$GITHUB_TOKEN" ]; then
    # Private repo: resolve the asset through the API, then download the binary.
    RELEASE_JSON="$(api_get "/releases/tags/$TAG")" || die "release $TAG not reachable"
    ASSET_URL="$(printf '%s' "$RELEASE_JSON" | json_asset_url)"
    [ -n "$ASSET_URL" ] || die "asset $ASSET_NAME not found in release $TAG"
    curl -fL --retry 3 --connect-timeout 15 --max-time 900 \
        -H "Accept: application/octet-stream" \
        -H "Authorization: Bearer $GITHUB_TOKEN" \
        -o "$PKG" "$ASSET_URL" || die "download failed"
else
    curl -fL --retry 3 --connect-timeout 15 --max-time 900 \
        -o "$PKG" \
        "https://github.com/$GITHUB_REPO/releases/download/$TAG/$ASSET_NAME" \
        || die "download failed (private repo? set GITHUB_TOKEN)"
fi

[ -s "$PKG" ] || die "downloaded file is empty"
tar -tzf "$PKG" >/dev/null || die "downloaded file is not a valid tar.gz"
# No "grep -q": with pipefail it can SIGPIPE tar and cause a false failure.
tar -tzf "$PKG" | grep -x './ezbookkeeping' >/dev/null || die "package has no backend binary"
log "downloaded $(du -h "$PKG" | cut -f1)"

# ---- preflight on the live install -----------------------------------------
[ -d "$INSTALL_DIR" ] || die "install dir does not exist: $INSTALL_DIR"
[ -f "$CONF_FILE" ] || die "real config not found: $CONF_FILE (refusing to break your install)"
[ -f "$INSTALL_DIR/ezbookkeeping" ] || die "binary not found: $INSTALL_DIR/ezbookkeeping"

TS="$(date +%Y%m%d-%H%M%S)"
mkdir -p "$BACKUP_DIR"
prune "$BACKUP_DIR" '.stage-*' 0   # leftovers from an interrupted run
APP_BACKUP="$BACKUP_DIR/app-$TS"

# ---- stop + back up --------------------------------------------------------
if systemctl is-active --quiet "$SERVICE_NAME"; then
    systemctl stop "$SERVICE_NAME"
    SERVICE_STOPPED=1
    log "service stopped"
fi

DB_BACKUP="$BACKUP_DIR/db-$TS.sql.gz"
DB_BACKUP_ERR="$TMP_DIR/db-dump.err"
if [ -n "$DB_BACKUP_CMD" ]; then
    # Fail-closed: no valid dump -> no deployment. cleanup() restarts the
    # service on any non-zero exit, so a broken mysqldump never means downtime.
    if ! bash -c "$DB_BACKUP_CMD" 2>"$DB_BACKUP_ERR" | gzip -9 > "$DB_BACKUP"; then
        rm -f -- "$DB_BACKUP"
        sed 's/^/  | /' "$DB_BACKUP_ERR" >&2 || true
        # By far the most common cause of a failed dump on an otherwise working
        # install: the dump account is a plain DML user. Say so instead of
        # leaving the operator to decode the mysqldump error above.
        log "hint: if mysqldump said 'FLUSH ... TABLES: Access denied', the dump account"
        log "      needs the global RELOAD (or FLUSH_TABLES) privilege; add PROCESS too,"
        log "      or pass --no-tablespaces, to avoid a later tablespace privilege error."
        log "      See the PRIVILEGES notes in deploy/ez-update.env.example."
        die "database backup failed (service was NOT modified and will be restarted)"
    fi
    [ -s "$DB_BACKUP" ] || die "database backup is empty"
    gzip -t "$DB_BACKUP" || die "database backup is not a valid gzip stream"
    # mysqldump can exit 0 *after* complaining about objects it could not read,
    # e.g. "Error ... when trying to dump masking policies". Such a dump is
    # silently INCOMPLETE, which is worse than having no backup at all, because
    # it would only be discovered during a rollback. Refuse it by default.
    if [ -s "$DB_BACKUP_ERR" ]; then
        log "mysqldump reported:"
        sed 's/^/  | /' "$DB_BACKUP_ERR"
    fi
    if grep -q '^mysqldump: Error' "$DB_BACKUP_ERR"; then
        if [ "${ALLOW_PARTIAL_DB_BACKUP:-0}" != "1" ]; then
            rm -f -- "$DB_BACKUP"
            die "database backup is incomplete (set ALLOW_PARTIAL_DB_BACKUP=1 to accept it anyway)"
        fi
        log "WARNING: continuing with a PARTIAL database backup (ALLOW_PARTIAL_DB_BACKUP=1)"
    fi
    log "database dumped to $DB_BACKUP"
elif [ -f "$SQLITE_DB_PATH" ]; then
    cp -a "$SQLITE_DB_PATH" "$BACKUP_DIR/sqlite-$TS.db"
    log "sqlite database copied to $BACKUP_DIR/sqlite-$TS.db"
elif [ "${ALLOW_NO_DB_BACKUP:-0}" = "1" ]; then
    log "WARNING: proceeding WITHOUT a database backup (ALLOW_NO_DB_BACKUP=1)"
else
    die "no database backup possible: set DB_BACKUP_CMD (MySQL/MariaDB) or check SQLITE_DB_PATH, or set ALLOW_NO_DB_BACKUP=1 to accept the risk"
fi

backup_items "$APP_BACKUP"
log "saved previous artifacts to $APP_BACKUP"

# ---- stage + install -------------------------------------------------------
STAGE="$(mktemp -d "$BACKUP_DIR/.stage-XXXXXX")"
tar -xzf "$PKG" -C "$STAGE"
[ -f "$STAGE/ezbookkeeping" ] || die "backend binary missing after extraction"

# Whitelist: conf/, data/, storage/, log/ inside the package are discarded.
# The real /opt/ez/conf/ezbookkeeping.ini can never be overwritten.
find "$STAGE" -mindepth 1 -maxdepth 1 \
    ! -name ezbookkeeping ! -name public ! -name templates ! -name LICENSE \
    -exec rm -rf -- {} +

install_items "$STAGE"
fix_selinux_contexts
rm -rf -- "$STAGE"
STAGE=""
log "replaced $(printf '%s ' "${ITEMS[@]}")in $INSTALL_DIR (conf/ data/ storage/ log/ untouched)"

# ---- start + health probe + rollback --------------------------------------
systemctl daemon-reload
if ! systemctl start "$SERVICE_NAME"; then
    journalctl -u "$SERVICE_NAME" -n 40 --no-pager || true
    rollback
fi
SERVICE_STOPPED=0

if ! BODY="$(probe)"; then
    log "health check failed after $((HEALTH_RETRIES * HEALTH_INTERVAL))s: $HEALTH_URL"
    journalctl -u "$SERVICE_NAME" -n 40 --no-pager || true
    rollback
fi

prune "$BACKUP_DIR" 'app-*' "$KEEP_BACKUPS"
prune "$BACKUP_DIR" 'db-*' "$KEEP_BACKUPS"
prune "$BACKUP_DIR" 'sqlite-*' "$KEEP_BACKUPS"

log "OK: $TAG is live ($ARCH)"
log "health: $BODY"
