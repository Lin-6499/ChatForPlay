#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
if [ "$(id -u)" -ne 0 ]; then
    printf '%s\n' 'Run this installer as root with sudo.'
    exit 1
fi
if [ -e /opt/linchat ] || [ -e /var/lib/linchat ] || [ -e /etc/systemd/system/linchat.service ] || [ -e /etc/nginx/sites-available/linchat ]; then
    printf '%s\n' 'A LinChat destination already exists. Inspect it before updating; this first-install script will not overwrite it.'
    exit 1
fi
if ss -lnt '( sport = :8787 )' | grep -q 'LISTEN'; then
    printf '%s\n' 'TCP 8787 is already in use.'
    exit 1
fi
nginx -t
install -d -m 0700 /var/backups/linchat
nginx -T > "/var/backups/linchat/nginx-before-$(date +%Y%m%d-%H%M%S).txt" 2>&1
if ! id linchat >/dev/null 2>&1; then
    useradd --system --user-group --home-dir /var/lib/linchat --shell /usr/sbin/nologin linchat
fi
install -d -m 0755 /opt/linchat/bin
install -d -m 0700 -o linchat -g linchat /var/lib/linchat
install -m 0755 linchat-linux-amd64 /opt/linchat/bin/linchat
install -m 0644 linchat.service /etc/systemd/system/linchat.service
install -d -m 0755 /var/www/linchat-acme/.well-known/acme-challenge
install -m 0644 nginx-bootstrap.conf /etc/nginx/sites-available/linchat
ln -s /etc/nginx/sites-available/linchat /etc/nginx/sites-enabled/linchat
nginx -t
systemctl daemon-reload
systemctl enable --now linchat
systemctl reload nginx
curl --fail --silent --show-error --retry 5 --retry-connrefused --retry-delay 1 http://127.0.0.1:8787/api/health
printf '\n%s\n' 'LinChat installed. Next: DNS, certificate issuance, HTTPS virtual host.'
