# Windows 上由用户自行安装 Npcap，安装包不分发 `Packet.dll`；依赖收拢在一处，便于以后替换

`easytier-core.exe` v2.6.4 静态链接了 `pnet`，后者在 Windows 上通过 `Packet.dll`（WinPcap/Npcap 的 Packet API：`PacketGetAdapterNames`、`PacketOpenAdapter` 等 11 个函数）访问网卡，而且是硬导入：缺了它进程根本起不来。官方 Windows 压缩包和 EasyTier 自己的 GUI 安装包都直接带上了 Npcap 的 `Packet.dll`（`easytier/third_party/x86_64/Packet.dll`，上游 issue EasyTier#2273 只讨论更新版本，没有涉及许可）。但 Npcap 的许可写明它"不是开源软件，未经 Nmap Project 特别许可不得再分发"，并建议开源项目"让用户自己下载安装 Npcap"；免费版每人限装 5 台，静默安装只有付费的 OEM 版才有。Wireshark 等项目要么让用户自装，要么拿到了专门授权。

我们的做法：Windows 安装包内置 `easytier-core.exe`、`easytier-cli.exe`、`wintun.dll`、`WinDivert64.sys`（都允许再分发），**不带 `Packet.dll`**。安装时检测 Npcap（`%SystemRoot%\System32\Npcap\Packet.dll`），没有就提示并给出 https://npcap.com 链接。后台服务启动 `easytier-core` 前检查 `Packet.dll` 能否找到，并把 `System32\Npcap`（不在 Windows 默认的 DLL 搜索路径里）加到子进程的 `PATH` 中；找不到时加入房间失败，错误码为 `engine_dependency_missing`，GUI 和 CLI 提示安装 Npcap。

对 Npcap 的全部了解只在两处：`cmd/mokoup2pd/main_windows.go` 的 `platformEngineOptions`（引擎选项 `RequiredLibraries`、`DLLSearchDirs`）和安装脚本 `packaging/windows/mokoup2p.nsi` 里的检测。依赖受限软件限制太多，这是过渡方案：以后换成不依赖 `Packet.dll` 的 EasyTier 版本（或自行构建去掉 `pnet` 数据链路依赖的 `easytier-core`），或者换成许可允许再分发的实现时，只需改这两处并删除本 ADR 的限制。

## Considered Options

- **像 EasyTier 一样直接打包 `Packet.dll`**：用户体验最好，但与 Npcap 许可的文字直接冲突。
- **安装时从 EasyTier 官方发布页下载压缩包**：我们不直接分发，但仍是在帮用户获取未经授权的分发物；安装必须联网并依赖 GitHub。
- **申请 Npcap 的开源再分发许可**：可以另外进行；拿到许可后改为打包，并相应更新本 ADR。

## Consequences

- Windows 用户需要多装一个 Npcap（免费版限 5 台机器）。Npcap 的安装界面不能静默跳过。
- 若用户卸载了 Npcap，加入房间时会得到明确的提示，而不是加载器错误。
- 升级 EasyTier 时检查 `easytier-core.exe` 是否仍导入 `packet.dll`（`objdump -p easytier-core.exe | grep -i packet`）；不再导入时去掉 `RequiredLibraries` 和安装包里的检测。
