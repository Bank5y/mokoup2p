#!/bin/sh
set -e
command -v systemctl >/dev/null 2>&1 || exit 0
[ -d /run/systemd/system ] || exit 0
systemctl daemon-reload || true
case "${1:-}" in
upgrade | 1) systemctl try-restart 'mokoup2pd@*.service' || true ;;
esac
