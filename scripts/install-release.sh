#!/usr/bin/env bash
set -euo pipefail
release="${1:?release directory required}"
case "$release" in /data/RevenueStatisticsSystem/releases/*) ;; *) echo 'invalid release path'; exit 1;; esac
root=/data/RevenueStatisticsSystem
python3 - "$release" <<'PY'
import hashlib,json,sys
from pathlib import Path
root=Path(sys.argv[1]).resolve()
for item in json.loads((root/'manifest.json').read_text(encoding='utf-8-sig')):
    path=(root/item['path']).resolve()
    if root not in path.parents or hashlib.sha256(path.read_bytes()).hexdigest()!=item['sha256']:
        raise SystemExit('release file verification failed')
print('release manifest verified')
PY
if ! id revenuestatistics >/dev/null 2>&1; then useradd --system --no-create-home --shell /sbin/nologin revenuestatistics; fi
install -d -m 700 -o revenuestatistics -g revenuestatistics "$root/config"
test -s "$root/config/dm-dsn"
python3 "$release/register-identity.py"
chown -R revenuestatistics:revenuestatistics "$root/config"
chmod 700 "$root/config"
chmod 600 "$root/config/"*
find "$release" -type d -exec chmod 755 {} \;
find "$release" -type f -exec chmod 644 {} \;
chmod 755 "$release/revenuestatistics"
install -d -m 755 "$root/backups"
stamp=$(date +%Y%m%d-%H%M%S)
systemctl show nginx --property=MainPID --value > "$root/backups/nginx-pid-$stamp.txt"
if test -L "$root/current"; then readlink -f "$root/current" > "$root/backups/previous-release-$stamp.txt"; fi
if test -e /etc/nginx/conf.d/revenuestatistics-8027.conf; then cp -p /etc/nginx/conf.d/revenuestatistics-8027.conf "$root/backups/nginx-$stamp.conf"; fi
if test -e /etc/systemd/system/revenuestatistics.service; then cp -p /etc/systemd/system/revenuestatistics.service "$root/backups/service-$stamp.unit"; fi
ln -sfn "$release" "$root/current.next"
mv -Tf "$root/current.next" "$root/current"
install -m 644 "$release/deploy/revenuestatistics-8027.conf" /etc/nginx/conf.d/revenuestatistics-8027.conf
install -m 644 "$release/deploy/revenuestatistics.service" /etc/systemd/system/revenuestatistics.service
nginx -t
systemctl daemon-reload
systemctl enable revenuestatistics.service
systemctl restart revenuestatistics.service
systemctl reload nginx
if systemctl is-active --quiet firewalld; then
 firewall-cmd --add-port=8027/tcp
 firewall-cmd --permanent --add-port=8027/tcp
fi
for attempt in 1 2 3 4 5; do
 if curl --noproxy '*' --fail --silent http://127.0.0.1:8127/api/health; then break; fi
 sleep 1
done
systemctl is-active revenuestatistics.service
curl --noproxy '*' --fail --silent http://127.0.0.1:8027/api/health
printf '\nrelease=%s\n' "$release"
