#!/bin/sh
# Stop every daemon instance (leaving rooms, stopping easytier-core) before
# the files go. On upgrade (deb: "upgrade", rpm: $1 >= 1) restart instead.
set -e
command -v systemctl >/dev/null 2>&1 || exit 0
[ -d /run/systemd/system ] || exit 0
case "${1:-}" in
upgrade | 1 | 2) exit 0 ;;
esac
for unit in $(systemctl list-units --all --plain --no-legend 'mokoup2pd@*.service' | awk '{print $1}'); do
	systemctl disable --now "$unit" || true
done
