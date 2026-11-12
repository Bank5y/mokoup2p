#!/bin/sh
# Removes what install.sh installed. Saved state in /var/lib/mokoup2p is
# kept unless --purge is given.
set -eu
[ "$(id -u)" = 0 ] || { echo "请用 sudo 运行：sudo $0" >&2; exit 1; }
if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then
	for unit in $(systemctl list-units --all --plain --no-legend 'mokoup2pd@*.service' | awk '{print $1}'); do
		systemctl disable --now "$unit" || true
	done
fi
rm -rf /usr/lib/mokoup2p /usr/share/doc/mokoup2p
rm -f /usr/bin/mokoup2p /usr/bin/mokoup2p-gui /usr/lib/systemd/system/mokoup2pd@.service
command -v systemctl >/dev/null 2>&1 && systemctl daemon-reload 2>/dev/null || true
[ "${1:-}" = --purge ] && rm -rf /var/lib/mokoup2p
echo "已卸载 mokoup2p。"
