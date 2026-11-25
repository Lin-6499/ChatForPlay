#!/usr/bin/env bash
# Upgrade the existing LinChat systemd deployment with a verified local binary.
set -Eeuo pipefail
if [[ $# -ne 3 || ! "$2" =~ ^[a-fA-F0-9]{64}$ || ! "$3" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Usage: sudo bash update-cloud.sh /path/to/linchat-linux-amd64 SHA256 VERSION" >&2
  exit 2
fi
[[ $(id -u) -eq 0 ]] || { echo "Run this upgrade with sudo." >&2; exit 1; }
binary=$(readlink -f -- "$1")
expected_sha=$2
expected_version=$3
[[ -f "$binary" && -f /opt/linchat/bin/linchat && -d /var/lib/linchat ]] || {
  echo "Existing LinChat deployment or new binary was not found." >&2
  exit 1
}
actual_sha=$(sha256sum -- "$binary" | cut -d ' ' -f 1)
[[ "${actual_sha,,}" == "${expected_sha,,}" ]] || { echo "Binary SHA-256 mismatch." >&2; exit 1; }
exec 9>/run/lock/linchat-update.lock
flock -n 9 || { echo "Another LinChat upgrade is running." >&2; exit 1; }
install -d -m 0700 /var/backups/linchat
backup=$(mktemp -d "/var/backups/linchat/update-$(date +%Y%m%d-%H%M%S)-XXXXXX")
install -m 0755 /opt/linchat/bin/linchat "$backup/linchat.previous"
recover() {
  code=$?
  trap - ERR INT TERM
  [[ "$code" -ne 0 ]] || code=1
  echo "Upgrade failed; restoring the previous program. Backup: $backup" >&2
  systemctl stop linchat.service || true
  install -m 0755 "$backup/linchat.previous" /opt/linchat/bin/linchat
  systemctl start linchat.service || true
  # The migration only adds a table/index, so the old program can still read the
  # data. Do not overwrite the database and lose messages accepted after startup.
  exit "${code:-1}"
}
trap recover ERR INT TERM
systemctl stop linchat.service
tar -C /var/lib/linchat -czpf "$backup/data.tar.gz" .
install -m 0755 "$binary" /opt/linchat/bin/.linchat.new
mv -f /opt/linchat/bin/.linchat.new /opt/linchat/bin/linchat
systemctl start linchat.service
curl --silent --show-error --fail --retry 15 --retry-delay 1 --retry-connrefused --retry-all-errors \
  --max-time 3 http://127.0.0.1:8787/api/health > "$backup/health.json"
python3 - "$backup/health.json" "$expected_version" <<'PY'
import json, sys
with open(sys.argv[1]) as f:
    health = json.load(f)
assert health.get("ok") is True and health.get("version") == sys.argv[2], health
PY
systemctl is-active --quiet linchat.service
trap - ERR INT TERM
printf 'LinChat %s is healthy. Previous binary and complete data backup: %s\n' "$expected_version" "$backup"
