#!/bin/sh
# Installs mokoup2p from the unpacked tar.gz and registers the systemd
# service for the desktop user:  sudo ./install.sh  (or sudo ./install.sh UID)
set -eu
cd "$(dirname "$0")"
[ "$(id -u)" = 0 ] || { echo "请用 sudo 运行：sudo $0" >&2; exit 1; }
uid=${1:-${SUDO_UID:-}}

install -d /usr/lib/mokoup2p /usr/share/doc/mokoup2p
install -m 0755 mokoup2pd easytier-core easytier-cli /usr/lib/mokoup2p/
install -m 0755 mokoup2p /usr/bin/mokoup2p
[ -f mokoup2p-gui ] && install -m 0755 mokoup2p-gui /usr/bin/mokoup2p-gui
install -m 0644 mokoup2pd@.service /usr/lib/systemd/system/mokoup2pd@.service
cp -r doc/. /usr/share/doc/mokoup2p/

if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then
	systemctl daemon-reload
	if [ -n "$uid" ] && [ "$uid" != 0 ]; then
		systemctl enable --now "mokoup2pd@$uid.service"
		echo "已为 UID $uid 启用后台服务 mokoup2pd@$uid"
	else
		echo "请为桌面用户启用后台服务：sudo systemctl enable --now mokoup2pd@<UID>"
	fi
fi
if [ -f mokoup2p-gui ] && command -v ldd >/dev/null 2>&1 && ldd /usr/bin/mokoup2p-gui 2>/dev/null | grep -q "not found"; then
	echo "提示：图形界面缺少运行库，请安装 GTK3 和 WebKit2GTK 4.1，例如："
	echo "  Debian/Ubuntu: sudo apt install libgtk-3-0 libwebkit2gtk-4.1-0"
	echo "  Fedora:        sudo dnf install gtk3 webkit2gtk4.1"
fi
echo "安装完成。运行 mokoup2p status 查看状态。"
