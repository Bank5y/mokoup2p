# Windows 客户端

## 组成

| 部分 | 说明 |
| --- | --- |
| `mokoup2pd.exe` | Windows 服务 `mokoup2p`（LocalSystem，自动启动，失败后自动重启）。`install [-user 用户名]` 注册并启动（已注册时就地升级，沿用原来允许的用户），`uninstall` 停止并删除，`run` 在控制台运行（调试）。 |
| IPC | 命名管道 `\\.\pipe\mokoup2p`。ACL 为 `D:P(A;;GA;;;SY)(A;;GRGW;;;<用户 SID>)`：只有 SYSTEM 和安装时指定的用户能连接。服务以 `-ipc-user <SID>` 参数注册。 |
| 状态 | `%ProgramData%\mokoup2p\state.json`（刷新令牌、设备标识），日志 `mokoup2pd.log`。 |
| `easytier-core.exe` | 锁定的 v2.6.4，未修改，作为子进程运行，放进一个 "关闭即终止" 的作业对象：服务进程无论怎样退出，子进程都会随之结束；Wintun 网卡随进程消失。启动时若 PID 文件指向的进程仍是 `easytier-core.exe`，先将其结束（残留清理）。 |
| GUI | Wails 在 Windows 上使用 WebView2，不需要 cgo；托盘可用，点 × 隐藏到托盘。 |

## 第三方文件与许可

EasyTier 的 Windows 压缩包包含：

- `wintun.dll`：Wintun 预编译二进制许可，可原样再分发（需附带其 LICENSE.txt）。
- `WinDivert64.sys`：LGPLv3 / GPLv2，广播修复（#13）使用。
- `Packet.dll`：**Npcap 的库，其许可不允许未经许可再分发**。`easytier-core.exe` v2.6.4 硬依赖它（上游 issue EasyTier#2273）。

在这个问题解决之前，安装包不能内置 `Packet.dll`。可选方案：用户自行安装 Npcap；安装时从 EasyTier 官方发布页下载压缩包到用户机器上（我们不再分发它）；或获得 Npcap 的再分发许可。

## 手动验证（#12 验收）

以下步骤无法在 Linux 开发环境中自动执行，需要在真实的 Windows 10/11 x64 机器上完成：

1. 以管理员身份运行 `mokoup2pd.exe install`。`sc query mokoup2p` 显示 RUNNING；`%ProgramData%\mokoup2p\mokoup2pd.log` 有 `listening on \\.\pipe\mokoup2p`。
2. 以安装用户运行 `mokoup2p.exe status`：能连上后台服务。换一个本机用户运行：连接被拒绝。
3. `mokoup2p.exe server …`、`login`、`join <房间号>`，同一房间里有一台 Linux 设备：
   - 两边互相 `ping` 对方的虚拟 IP；
   - `mokoup2p.exe members` 显示对方为"直连"或"中转"以及延迟。
4. 在任务管理器里结束 `mokoup2pd.exe`：`easytier-core.exe` 随之消失，网络适配器列表中没有残留的 `mokoup2p0`；服务自动重启后不会自动回到房间。
5. 运行 GUI：登录、加入房间、成员列表、点 × 隐藏到托盘仍在房间中、托盘"退出"离开房间。
6. `mokoup2pd.exe uninstall`：服务删除，虚拟网卡消失。

## 广播修复（#13 验收）

后台服务在 Windows 上给 `easytier-core.exe` 加上 `--enable-udp-broadcast-relay true`（见 ADR-0003），需要 `WinDivert64.sys` 与 `easytier-core.exe` 放在一起。

1. Windows 设备 A 和 Linux 或 Windows 设备 B 进入同一房间。
2. 在 B 上监听 UDP 端口并打印来源地址，例如 Linux：`socat -u UDP4-RECVFROM:9999,reuseaddr,fork SYSTEM:'echo $SOCAT_PEERADDR'`。
3. 在 A 上从 `0.0.0.0` 向 `255.255.255.255:9999` 发送广播（例如 PowerShell：`$u=New-Object Net.Sockets.UdpClient; $u.EnableBroadcast=$true; $b=[Text.Encoding]::ASCII.GetBytes('hi'); $u.Send($b,$b.Length,'255.255.255.255',9999)`）。
   B 应收到来源为 **A 的虚拟 IP** 的包（v2.6.4 下每个包会到两次，属已知问题）。
4. 同一时间物理局域网上的其他机器也应照常收到这个广播（修复不拦截原始包）。
5. A 离开房间后再发：B 不再收到。
6. 用至少一款依赖 UDP 受限广播发现房间的局域网游戏（或上面的测试工具）在 A 与 B 之间互相发现。

## 覆盖安装（#18 验收）

安装包可以直接覆盖安装已有版本：先结束 GUI、停止后台服务，替换文件后由 `mokoup2pd.exe install` 就地更新服务，**沿用原服务允许连接的用户**（不再取运行安装程序的用户）。`/S` 静默安装不弹任何对话框；加上 `/relaunch` 时，装完后以登录用户身份（不提权）重新打开 GUI。

1. 装好旧版本并登录；GUI 保持打开。双击新版本安装包覆盖安装：安装成功，不报文件被占用；GUI 被关闭；`sc qc mokoup2p` 的 `BINARY_PATH_NAME` 中 `-ipc-user` 与升级前相同；完成页勾选"启动 mokoup2p"打开的 GUI 不是管理员权限（任务管理器"提升的"一列为"否"），且无需重新登录。
2. 用标准用户 A 安装（UAC 输入管理员 B 的凭据）。升级前后在 A 下运行 `mokoup2p.exe status` 都能连上后台服务；在 B 下运行被拒绝。
3. 全新机器（没有 `mokoup2p` 服务）安装：服务允许的是运行安装程序的用户，和以前一样。
4. 卸载 Npcap 后以管理员运行 `setup.exe /S`：全程没有任何对话框，退出码为 0，服务为 RUNNING。
5. 以管理员运行 `setup.exe /S /relaunch`：装完后 GUI 以登录用户身份、非提权打开。

## 一键更新（#21 验收）

安装版 GUI 启动时检查 GitHub Releases。发现新版本后，点横幅上的"更新"，确认后 GUI 会依次：下载 `setup.exe` 到 `%TEMP%\mokoup2p-update\`，用同一 Release 的 `SHA256SUMS` 校验，经 UAC 以 `/S /relaunch` 运行安装包，然后自己退出；安装完成后 GUI 以原用户身份重新打开。下载和校验的逻辑有自动测试，以下步骤需在真机上验证（准备一个比本机版本号更高的 Release，例如在 fork 仓库里发布，并用 `MOKOUP2P_REPO` 构建本机版本）：

1. 以标准用户登录并加入房间，打开 GUI：顶部出现"发现新版本"横幅。点"更新"：显示"会断开当前房间"的确认。
2. 点"下载并安装"，在 UAC 中输入管理员凭据：GUI 退出，片刻后重新打开，非提权；`mokoup2p.exe status` 显示新版本，已登录、不在房间中，可以重新加入。
3. 重复第 1 步，在 UAC 中点"否"：GUI 留在原处，提示"已取消更新"，仍在房间中。
4. 把 Release 里的 `SHA256SUMS` 换成错误的哈希再试：提示"安装包校验失败"，不弹 UAC，`%TEMP%\mokoup2p-update\` 中没有安装包。
5. Linux 或绿色版上点"更新"：只打开 Release 页面。

## 绿色版（#22 验收）

`mokoup2p-<版本>-windows-x64-portable.zip` 与安装版是同样的程序，多一个标记文件 `portable`。GUI 看到它时：

- 若本机已有 `mokoup2p` 服务（装了安装版），提示使用安装版并退出；
- 若命名管道上已有后台服务应答（绿色版已在运行），直接进入，单实例锁会把已打开的窗口调出来；
- 否则检查 exe 旁的 `data\` 可写，经 UAC 隐藏启动 `mokoup2pd.exe portable -ipc-user <当前用户 SID> -state-dir <data> -parent-pid <GUI 进程>`，等它应答后打开窗口。

这个后台进程只允许当前用户连接管道，状态和日志写在 `data\`，GUI 进程结束时随之退出（离开房间，`easytier-core.exe` 和虚拟网卡一起清理）。GUI 启动时检测 Npcap（安装版也一样），缺少时在顶部提示并给出下载按钮。

1. 在没有安装版的机器上，把 zip 解压到桌面，双击 `mokoup2p-gui.exe`：弹一次 UAC；确认后能登录、加入房间、与另一台设备互 ping。`data\` 下有 `state.json` 和 `mokoup2pd.log`。
2. 再次双击 `mokoup2p-gui.exe`：不弹 UAC，已有窗口被调出。
3. 托盘"退出"：任务管理器里 `mokoup2pd.exe`、`easytier-core.exe` 都消失，网络适配器中没有残留 `mokoup2p0`。在任务管理器里直接结束 `mokoup2p-gui.exe`，结果相同。
4. 重新打开：无需重新登录（`data\state.json` 保留），不会自动回到房间。把整个文件夹拷到另一台机器上运行：同样无需重新登录。
5. UAC 点"否"：提示需要管理员权限后退出，没有残留进程。
6. 把文件夹放到 `C:\Program Files\` 下运行：提示数据目录不可写并退出，没有写入 `%ProgramData%`。
7. 在装有安装版的机器上运行绿色版：提示"已安装 mokoup2p，请使用安装版"并退出，不弹 UAC。
8. 运行前后对比 `sc query mokoup2p`（不存在）和 `HKLM\Software\mokoup2p`（不存在）：没有变化。
9. 卸载 Npcap 后打开 GUI（安装版和绿色版各一次）：顶部提示缺少 Npcap，"下载 Npcap"打开 npcap.com。
10. 有新版本时，绿色版的"更新"按钮只打开 Release 页面。
