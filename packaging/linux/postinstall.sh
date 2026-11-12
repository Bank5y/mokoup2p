#!/bin/sh
# Register the systemd unit. When installed through sudo, enable the daemon
# for the user who ran sudo; otherwise say how to.
set -e
command -v systemctl >/dev/null 2>&1 || exit 0
[ -d /run/systemd/system ] || exit 0
systemctl daemon-reload || true
uid=${SUDO_UID:-}
if [ -n "$uid" ] && [ "$uid" != 0 ]; then
	systemctl enable --now "mokoup2pd@$uid.service" || true
	echo "mokoup2p: 已为 UID $uid 启用后台服务 mokoup2pd@$uid"
else
	echo "mokoup2p: 请为桌面用户启用后台服务：sudo systemctl enable --now mokoup2pd@\$(id -u)"
fi
