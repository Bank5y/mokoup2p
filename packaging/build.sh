#!/bin/sh
# Builds the client packages into dist/:
#   mokoup2p-VERSION-linux-{amd64,arm64}.tar.gz   (with install.sh / uninstall.sh)
#   mokoup2p_VERSION_{amd64,arm64}.deb, mokoup2p-VERSION.{x86_64,aarch64}.rpm
#   mokoup2p-VERSION-windows-x64-setup.exe        (NSIS)
#
# Usage: packaging/build.sh [VERSION]
# Needs Go, npm, curl, unzip, and docker (for makensis and the Linux GUI).
#   MOKOUP2P_LINUX_GUI=0   skip the Linux GUI (packages then hold daemon + CLI)
#   MOKOUP2P_WINDOWS=0     skip the Windows installer
# The Linux GUI is built for amd64 only (cgo); arm64 packages hold daemon + CLI.
set -eu
root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"
. packaging/versions.env
version=${1:-$(git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo 0.0.0)}
case "$version" in [0-9]*) ;; *) version=0.0.0+$version ;; esac
build=$root/build
dist=$root/dist
rm -rf "$build/stage" "$dist"
mkdir -p "$build/stage" "$dist"
export MOKOUP2P_CACHE=$build/cache
ldflags="-s -w"

frontend() {
	if [ ! -f gui/frontend/dist/index.html ]; then
		(cd gui/frontend && npm ci && npm run build)
	fi
}

docs() { # DIR
	mkdir -p "$1"
	cp README.md THIRD_PARTY_NOTICES.md docs/deploy.md "$1/"
}

nfpm() {
	go run "github.com/goreleaser/nfpm/v2/cmd/nfpm@$NFPM_VERSION" "$@"
}

linux() { # ARCH
	arch=$1
	stage=$build/stage/linux-$arch
	mkdir -p "$stage"
	echo "== linux/$arch"
	GOOS=linux GOARCH=$arch CGO_ENABLED=0 go build -trimpath -ldflags "$ldflags" -o "$stage/" ./cmd/mokoup2pd ./cmd/mokoup2p
	packaging/fetch-easytier.sh linux "$arch" "$stage"
	install -m 0644 deploy/systemd/mokoup2pd@.service "$stage/"
	docs "$stage/doc"
	mv "$stage/EasyTier-LICENSE.txt" "$stage/doc/"
	gui=""
	if [ "$arch" = amd64 ] && [ "${MOKOUP2P_LINUX_GUI:-1}" = 1 ]; then
		frontend
		docker build -q -t mokoup2p-gui-builder -f packaging/linux/Dockerfile.gui packaging/linux >/dev/null
		docker run --rm -v "$root:/src" -v "$(go env GOMODCACHE):/go/pkg/mod" -u "$(id -u):$(id -g)" \
			-e HOME=/tmp -e GOCACHE=/tmp/gocache -e GOFLAGS=-mod=mod \
			mokoup2p-gui-builder go build -trimpath -tags desktop,production,webkit2_41 -ldflags "$ldflags" \
			-o "/src/build/stage/linux-$arch/mokoup2p-gui" .
		gui="  - src: $stage/mokoup2p-gui
    dst: /usr/bin/mokoup2p-gui
    file_info: { mode: 0755 }"
	fi

	# tar.gz with install script
	name=mokoup2p-$version-linux-$arch
	rm -rf "$build/$name"
	cp -r "$stage" "$build/$name"
	install -m 0755 packaging/linux/install.sh packaging/linux/uninstall.sh "$build/$name/"
	tar -C "$build" -czf "$dist/$name.tar.gz" "$name"

	# .deb and .rpm
	cfg=$build/nfpm-$arch.yaml
	awk -v gui="$gui" '/^\$\{GUI_CONTENTS\}$/ { if (gui != "") print gui; next } { print }' packaging/linux/nfpm.yaml |
		sed -e "s|\${ARCH}|$arch|g" -e "s|\${VERSION}|$version|g" -e "s|\${STAGE}|$stage|g" \
			-e "s|\${SCRIPTS}|$root/packaging/linux|g" >"$cfg"
	nfpm pkg --packager deb --config "$cfg" --target "$dist/"
	nfpm pkg --packager rpm --config "$cfg" --target "$dist/"
}

windows() {
	stage=$build/stage/windows-amd64
	mkdir -p "$stage"
	echo "== windows/amd64"
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$ldflags" -o "$stage/" ./cmd/mokoup2pd ./cmd/mokoup2p
	frontend
	(cd gui && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -tags desktop,production \
		-ldflags "$ldflags -H windowsgui" -o "$stage/mokoup2p-gui.exe" .)
	packaging/fetch-easytier.sh windows amd64 "$stage"
	# NSIS shows this as the licence page; CRLF for Notepad.
	sed 's/$/\r/' THIRD_PARTY_NOTICES.md >"$stage/THIRD_PARTY_NOTICES.txt"
	docker build -q -t mokoup2p-nsis packaging/windows >/dev/null
	cp packaging/windows/mokoup2p.nsi "$build/stage/"
	docker run --rm -v "$build/stage:/work" -u "$(id -u):$(id -g)" mokoup2p-nsis \
		-V2 -DVERSION="$version" -DSTAGE=windows-amd64 -DOUTFILE="mokoup2p-$version-windows-x64-setup.exe" mokoup2p.nsi
	mv "$build/stage/mokoup2p-$version-windows-x64-setup.exe" "$dist/"
}

linux amd64
linux arm64
[ "${MOKOUP2P_WINDOWS:-1}" = 1 ] && windows
(cd "$dist" && sha256sum * >SHA256SUMS)
echo
ls -l "$dist"
