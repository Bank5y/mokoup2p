; mokoup2p Windows installer (NSIS 3, Unicode). build.sh stages every file
; in STAGE and passes VERSION:
;   makensis -DVERSION=0.1.0 -DSTAGE=stage -DOUTFILE=mokoup2p-0.1.0-windows-x64-setup.exe mokoup2p.nsi
;
; Bundled: mokoup2pd.exe, mokoup2p.exe, mokoup2p-gui.exe, the pinned
; unmodified easytier-core.exe / easytier-cli.exe (LGPL-3.0), wintun.dll and
; WinDivert64.sys. NOT bundled: Npcap's Packet.dll, which may not be
; redistributed; the installer checks for Npcap (docs/adr/0004).

Unicode true
!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "x64.nsh"

!ifndef VERSION
  !define VERSION "0.0.0"
!endif
!ifndef STAGE
  !define STAGE "stage"
!endif
!ifndef OUTFILE
  !define OUTFILE "mokoup2p-${VERSION}-windows-x64-setup.exe"
!endif

!define PRODUCT "mokoup2p"
!define UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT}"
!define NPCAP_URL "https://npcap.com/#download"

Name "${PRODUCT} ${VERSION}"
OutFile "${OUTFILE}"
InstallDir "$PROGRAMFILES64\${PRODUCT}"
InstallDirRegKey HKLM "Software\${PRODUCT}" "InstallDir"
RequestExecutionLevel admin
SetCompressor /SOLID lzma

!define MUI_FINISHPAGE_RUN "$INSTDIR\mokoup2p-gui.exe"
!define MUI_FINISHPAGE_RUN_TEXT "启动 mokoup2p"
!insertmacro MUI_PAGE_LICENSE "${STAGE}\THIRD_PARTY_NOTICES.txt"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "SimpChinese"

Function .onInit
  ${IfNot} ${RunningX64}
    MessageBox MB_ICONSTOP "mokoup2p 仅支持 64 位 Windows。"
    Abort
  ${EndIf}
  SetRegView 64
FunctionEnd

; Npcap provides Packet.dll, which easytier-core needs (see ADR-0004).
Function CheckNpcap
  ${IfNot} ${FileExists} "$SYSDIR\Npcap\Packet.dll"
    MessageBox MB_YESNO|MB_ICONEXCLAMATION "未检测到 Npcap。$\r$\n$\r$\nmokoup2p 的网络组件需要 Npcap 才能工作（Npcap 的许可不允许我们随安装包分发）。$\r$\n安装完成后请从 npcap.com 下载安装 Npcap，安装时保持默认选项即可。$\r$\n$\r$\n现在打开 Npcap 下载页面吗？" IDNO done
    ExecShell "open" "${NPCAP_URL}"
  done:
  ${EndIf}
FunctionEnd

Section "mokoup2p" SecMain
  SectionIn RO
  SetOutPath "$INSTDIR"

  ; Upgrading: stop and remove the old service first (it leaves its room).
  ${If} ${FileExists} "$INSTDIR\mokoup2pd.exe"
    nsExec::ExecToLog '"$INSTDIR\mokoup2pd.exe" uninstall'
  ${EndIf}

  File "${STAGE}\mokoup2pd.exe"
  File "${STAGE}\mokoup2p.exe"
  File "${STAGE}\mokoup2p-gui.exe"
  File "${STAGE}\easytier-core.exe"
  File "${STAGE}\easytier-cli.exe"
  File "${STAGE}\wintun.dll"
  File "${STAGE}\WinDivert64.sys"
  SetOutPath "$INSTDIR\licenses"
  File "${STAGE}\THIRD_PARTY_NOTICES.txt"
  File "${STAGE}\wintun-LICENSE.txt"
  File "${STAGE}\EasyTier-LICENSE.txt"
  SetOutPath "$INSTDIR"

  ; Register and start the service for the user running the installer
  ; (mokoup2pd install defaults to the current user's SID).
  nsExec::ExecToLog '"$INSTDIR\mokoup2pd.exe" install'
  Pop $0
  ${If} $0 != 0
    MessageBox MB_ICONEXCLAMATION "注册后台服务失败（$0）。可以稍后以管理员身份运行：$\r$\n$\"$INSTDIR\mokoup2pd.exe$\" install"
  ${EndIf}

  CreateDirectory "$SMPROGRAMS\${PRODUCT}"
  CreateShortcut "$SMPROGRAMS\${PRODUCT}\mokoup2p.lnk" "$INSTDIR\mokoup2p-gui.exe"
  CreateShortcut "$SMPROGRAMS\${PRODUCT}\卸载 mokoup2p.lnk" "$INSTDIR\uninstall.exe"
  CreateShortcut "$DESKTOP\mokoup2p.lnk" "$INSTDIR\mokoup2p-gui.exe"

  WriteUninstaller "$INSTDIR\uninstall.exe"
  WriteRegStr HKLM "Software\${PRODUCT}" "InstallDir" "$INSTDIR"
  WriteRegStr HKLM "${UNINST_KEY}" "DisplayName" "${PRODUCT}"
  WriteRegStr HKLM "${UNINST_KEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKLM "${UNINST_KEY}" "Publisher" "${PRODUCT}"
  WriteRegStr HKLM "${UNINST_KEY}" "DisplayIcon" "$INSTDIR\mokoup2p-gui.exe"
  WriteRegStr HKLM "${UNINST_KEY}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegDWORD HKLM "${UNINST_KEY}" "NoModify" 1
  WriteRegDWORD HKLM "${UNINST_KEY}" "NoRepair" 1

  Call CheckNpcap
SectionEnd

Section "Uninstall"
  SetRegView 64
  ; Close the GUI, then stop (leaving the room) and delete the service.
  nsExec::ExecToLog 'taskkill /F /IM mokoup2p-gui.exe'
  nsExec::ExecToLog '"$INSTDIR\mokoup2pd.exe" uninstall'

  Delete "$INSTDIR\mokoup2pd.exe"
  Delete "$INSTDIR\mokoup2p.exe"
  Delete "$INSTDIR\mokoup2p-gui.exe"
  Delete "$INSTDIR\easytier-core.exe"
  Delete "$INSTDIR\easytier-cli.exe"
  Delete "$INSTDIR\wintun.dll"
  Delete "$INSTDIR\WinDivert64.sys"
  RMDir /r "$INSTDIR\licenses"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"

  Delete "$SMPROGRAMS\${PRODUCT}\mokoup2p.lnk"
  Delete "$SMPROGRAMS\${PRODUCT}\卸载 mokoup2p.lnk"
  RMDir "$SMPROGRAMS\${PRODUCT}"
  Delete "$DESKTOP\mokoup2p.lnk"

  ; Saved state (login, device identity) in %ProgramData%\mokoup2p goes too.
  SetShellVarContext all
  RMDir /r "$APPDATA\${PRODUCT}"
  SetShellVarContext current

  DeleteRegKey HKLM "${UNINST_KEY}"
  DeleteRegKey HKLM "Software\${PRODUCT}"
SectionEnd
