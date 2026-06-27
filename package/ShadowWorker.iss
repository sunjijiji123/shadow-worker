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

[UninstallRun]
Filename: "taskkill"; Parameters: "/F /IM {#MyAppExeName}"; Flags: runhidden
Filename: "taskkill"; Parameters: "/F /IM {#MyAppBackendName}"; Flags: runhidden
