# Third-party software in mokoup2p client packages

mokoup2p's own code is in this repository. The client packages also contain
the following third-party programs, **unmodified**, as separate files.

## EasyTier v2.6.4 (`easytier-core`, `easytier-cli`)

- Copyright © the EasyTier contributors. Licensed under the **GNU Lesser General Public License v3.0** (LGPL-3.0); the full text is shipped as `EasyTier-LICENSE.txt`.
- Source code: https://github.com/EasyTier/EasyTier/tree/v2.6.4 (the binaries are the official release assets of that tag; their SHA-256 sums are in `packaging/versions.env`).
- mokoup2p runs `easytier-core` as a separate child process and talks to it only through its command line, environment and local RPC (`easytier-cli`). It neither links nor modifies EasyTier, so you may replace these binaries with your own build of the same version (put them next to `mokoup2pd`, or point `MOKOUP2P_EASYTIER_DIR` at them).

## Wintun 0.14.1 (`wintun.dll`, Windows only)

- Copyright © WireGuard LLC. Redistributed unmodified under the Wintun Prebuilt Binaries License; the full text is shipped as `wintun-LICENSE.txt`. https://www.wintun.net/

## WinDivert 2.x driver (`WinDivert64.sys`, Windows only)

- Copyright © basil00. Dual licensed **LGPL-3.0 / GPL-2.0**; used by EasyTier's UDP broadcast relay. Source: https://github.com/basil00/WinDivert. The WinDivert user-mode library is linked statically into `easytier-core.exe` by EasyTier.

## Not included: Npcap (`Packet.dll`, Windows only)

`easytier-core.exe` v2.6.4 needs Npcap's `Packet.dll`. Npcap's license does not allow redistribution, so it is **not** shipped; install Npcap yourself from https://npcap.com (free for up to 5 systems under Npcap's own license). See `docs/adr/0004-npcap-dependency.md`.

## Go modules linked into mokoup2p binaries

The Go binaries statically link open-source Go modules (BSD-3-Clause, MIT, Apache-2.0 and similar permissive licences), including `golang.org/x/*`, `modernc.org/sqlite`, `github.com/gorilla/websocket`, `github.com/Microsoft/go-winio`, `fyne.io/systray` and `github.com/wailsapp/wails/v2`. Their licence texts are in the respective module sources (`go mod download -json` lists them).
