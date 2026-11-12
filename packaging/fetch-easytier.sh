#!/bin/sh
# fetch-easytier.sh OS ARCH OUTDIR: download the pinned, unmodified EasyTier
# release for OS/ARCH (linux amd64|arm64, windows amd64), verify its sha256
# and put the files we ship in OUTDIR. On Windows, Npcap's Packet.dll is
# left out on purpose: we may not redistribute it (docs/adr/0004).
set -eu
os=$1 arch=$2 out=$3
here=$(cd "$(dirname "$0")" && pwd)
. "$here/versions.env"
cache=${MOKOUP2P_CACHE:-$here/../build/cache}
mkdir -p "$cache" "$out"

case "$os/$arch" in
linux/amd64) asset=easytier-linux-x86_64 ;;
linux/arm64) asset=easytier-linux-aarch64 ;;
windows/amd64) asset=easytier-windows-x86_64 ;;
*) echo "unsupported platform $os/$arch" >&2; exit 2 ;;
esac
eval "sum=\$EASYTIER_SHA256_${os}_${arch}"

fetch() { # URL FILE SHA256
	if [ ! -f "$2" ] || ! echo "$3  $2" | sha256sum -c - >/dev/null 2>&1; then
		curl -fsSL -o "$2.part" "$1"
		mv "$2.part" "$2"
	fi
	echo "$3  $2" | sha256sum -c - >/dev/null || { echo "checksum mismatch: $2" >&2; exit 1; }
}

zip=$cache/$asset-$EASYTIER_VERSION.zip
fetch "https://github.com/EasyTier/EasyTier/releases/download/$EASYTIER_VERSION/$asset-$EASYTIER_VERSION.zip" "$zip" "$sum"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
unzip -q "$zip" -d "$tmp"
src=$tmp/$asset
if [ "$os" = windows ]; then
	for f in easytier-core.exe easytier-cli.exe wintun.dll WinDivert64.sys; do
		install -m 0644 "$src/$f" "$out/$f"
	done
	wzip=$cache/wintun-$WINTUN_VERSION.zip
	fetch "https://www.wintun.net/builds/wintun-$WINTUN_VERSION.zip" "$wzip" "$WINTUN_SHA256"
	unzip -q -o -j "$wzip" wintun/LICENSE.txt -d "$tmp/wintun"
	install -m 0644 "$tmp/wintun/LICENSE.txt" "$out/wintun-LICENSE.txt"
else
	install -m 0755 "$src/easytier-core" "$out/easytier-core"
	install -m 0755 "$src/easytier-cli" "$out/easytier-cli"
fi
lic=$cache/EasyTier-LICENSE-$EASYTIER_VERSION.txt
fetch "https://raw.githubusercontent.com/EasyTier/EasyTier/$EASYTIER_VERSION/LICENSE" "$lic" "$EASYTIER_LICENSE_SHA256"
install -m 0644 "$lic" "$out/EasyTier-LICENSE.txt"
echo "EasyTier $EASYTIER_VERSION ($os/$arch) -> $out"
