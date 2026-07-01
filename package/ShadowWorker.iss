; Shadow Worker Windows Installer (Inno Setup)

#define MyAppName "Shadow Worker"
#ifndef APP_VERSION
  #define APP_VERSION "0.0.0.0"
#endif
#define MyAppPublisher "Shadow Worker Team"
#define MyAppExeName "shadow-worker-client.exe"
#define MyAppBackendName "shadow-worker.exe"

[Setup]
AppId={{B4A8C1D2-E5F6-4A7B-8C9D-0E1F2A3B4C5D}
AppName={#MyAppName}
AppVersion={#APP_VERSION}
AppPublisher={#MyAppPublisher}
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=no
PrivilegesRequired=admin
OutputDir=..\dist
OutputBaseFilename=ShadowWorker-{#APP_VERSION}-setup
SetupIconFile=..\client\assets\app.ico
Compression=lzma
SolidCompression=yes
WizardStyle=modern

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"
Name: "autostart"; Description: "开机自启"; GroupDescription: "附加任务:"

[Files]
Source: "..\dist\bin\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{group}\{cm:UninstallProgram,{#MyAppName}}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "启动 Shadow Worker"; Flags: nowait postinstall skipifsilent

[Registry]
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "{#MyAppName}"; ValueData: """{app}\{#MyAppExeName}"" --autostart"; Tasks: autostart

; 卸载时的进程清理由 [Code] 段的 PrepareToInstall / CurUninstallStepChanged 统一处理
; （单次 taskkill。覆盖安装的真正根因——MCP 子进程锁文件——已通过把 MCP 拆成
; 独立 exe shadow-worker-mcp.exe 根治，见 AGENTS.md 坑 50）。

[Code]
// 安装/卸载前清掉主后端 + 客户端进程，它们会被覆盖且 admin 权限安装器有权限杀。
// 注意：不杀 shadow-worker-mcp.exe —— 它正被外部 AI agent 持有（stdio MCP），
// 杀了会中断 agent 会话；且它跑在独立 exe 上，不在本次覆盖范围（不会锁文件）。
// MCP 拆分前曾用「循环杀 shadow-worker.exe」治理 MCP 子进程锁文件，但 agent 会
// 秒级重拉，治标失败；现已根治（MCP 独立 exe），无需再循环杀。

// 用 tasklist + findstr 检测进程是否在运行。findstr 命中（exit code 0）= 在跑。
function IsProcessRunning(const procName: String): Boolean;
var
  ResultCode: Integer;
begin
  Result := True;
  if Exec(ExpandConstant('{cmd}'), '/C tasklist /FI "IMAGENAME eq ' + procName +
     '" 2>nul | findstr /I "' + procName + '"', '', SW_HIDE,
     ewWaitUntilTerminated, ResultCode) then
    Result := (ResultCode = 0);
end;

// 单次强杀主后端 + 客户端（不杀 mcp exe，不循环）。
procedure KillShadowWorker;
var
  ResultCode: Integer;
begin
  // 先杀客户端（它会在 aboutToQuit 拉起/留下后端），再杀主后端。
  Exec(ExpandConstant('{cmd}'), '/C taskkill /F /IM {#MyAppExeName}',
       '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Exec(ExpandConstant('{cmd}'), '/C taskkill /F /IM {#MyAppBackendName}',
       '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Sleep(1000);  // 给句柄释放
end;

// 安装前（文件复制前）执行：清掉主后端 + 客户端才能覆盖 exe。
// 返回非空字符串 = 中断安装并显示该文案。
function PrepareToInstall(var NeedsRestart: Boolean): String;
begin
  NeedsRestart := False;
  KillShadowWorker;
  if not IsProcessRunning('{#MyAppBackendName}') and
     not IsProcessRunning('{#MyAppExeName}') then
    Result := ''
  else
    Result :=
      '仍有 Shadow Worker 进程在运行，无法覆盖安装。' + #13#10 + #13#10 +
      '请通过任务管理器手动结束 shadow-worker.exe 与 shadow-worker-client.exe 后重试。';
end;

// 卸载时同样清进程（删文件前）。
procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usUninstall then
    KillShadowWorker;
end;
