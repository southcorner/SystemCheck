#!/usr/bin/env bash
# SystemCheck backup: dumps the Postgres database and syncs the screenshot store.
# Usage: BACKUP_DIR=/backups ./backup.sh
# Reads DB_* and MinIO settings from deploy/server.env if present.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
[ -f "$HERE/server.env" ] && set -a && . "$HERE/server.env" && set +a

BACKUP_DIR="${BACKUP_DIR:-./backups}"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
DEST="$BACKUP_DIR/$STAMP"
mkdir -p "$DEST"

DB_USER="${DB_USER:-systemcheck}"
DB_NAME="${DB_NAME:-systemcheck}"
DB_HOST="${DB_HOST:-127.0.0.1}"
DB_PORT="${DB_PORT:-5432}"

echo "[*] Dumping database $DB_NAME -> $DEST/db.sql.gz"
PGPASSWORD="${DB_PASSWORD:-}" pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" "$DB_NAME" | gzip > "$DEST/db.sql.gz"

# Screenshot blobs. Filesystem store (dev): copy ./data/screenshots.
# MinIO (prod): use `mc mirror local/<bucket> "$DEST/screenshots"`.
if [ -d "./data/screenshots" ]; then
  echo "[*] Copying filesystem screenshot store"
  cp -a ./data/screenshots "$DEST/screenshots"
else
  echo "[!] No local screenshot dir; for MinIO run:"
  echo "    mc mirror local/${SC_S3_BUCKET:-screenshots} \"$DEST/screenshots\""
fi

echo "[+] Backup complete: $DEST"
echo
echo "Restore:"
echo "  gunzip -c $DEST/db.sql.gz | PGPASSWORD=... psql -h $DB_HOST -U $DB_USER $DB_NAME"
echo "  cp -a $DEST/screenshots ./data/screenshots   # or: mc mirror \"$DEST/screenshots\" local/<bucket>"
