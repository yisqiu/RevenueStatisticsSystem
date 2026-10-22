#!/usr/bin/env bash
# Activate a locally built update using the existing service, proxy and identity configuration.
set -euo pipefail
root=/data/RevenueStatisticsSystem
release="${1:?release directory required}"
case "$release" in "$root"/releases/*) ;; *) echo 'invalid release path'; exit 1;; esac
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
previous=$(readlink -f "$root/current")
test -x "$previous/revenuestatistics"
find "$release" -type d -exec chmod 755 {} \;
find "$release" -type f -exec chmod 644 {} \;
chmod 755 "$release/revenuestatistics"
mkdir -p "$root/backups"
printf '%s\n' "$previous" > "$root/backups/previous-release-$(date +%Y%m%d-%H%M%S).txt"
ln -sfn "$release" "$root/current.next"
mv -Tf "$root/current.next" "$root/current"
healthy=false
if systemctl restart revenuestatistics.service; then
  for attempt in 1 2 3 4 5; do
    if curl --noproxy '*' --fail --silent http://127.0.0.1:8127/api/health; then healthy=true; break; fi
    sleep 1
  done
fi
if [ "$healthy" != true ]; then
  ln -sfn "$previous" "$root/current.next"
  mv -Tf "$root/current.next" "$root/current"
  systemctl restart revenuestatistics.service
  echo 'Update failed; previous release restored'
  exit 1
fi
systemctl is-active revenuestatistics.service
printf '\nrelease=%s\n' "$release"
