; Shadow Worker Windows Installer (Inno Setup)

#define MyAppName "Shadow Worker"
#define MyAppVersion "0.1.0"
#define MyAppPublisher "Shadow Worker Team"
#define MyAppExeName "shadow-worker-client.exe"
#define MyAppBackendName "shadow-worker.exe"

[Setup]
AppId={{B4A8C1D2-E5F6-4A7B-8C9D-0E1F2A3B4C5D}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
DefaultDirName={autopf}\{#MyAppName}
DisableProgramGroupPage=no
PrivilegesRequired=admin
OutputDir=..\dist
OutputBaseFilename=ShadowWorker-{#MyAppVersion}-setup
SetupIconFile=
Compression=lzma
SolidCompression=yes
WizardStyle=modern

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked
Name: "autostart"; Description: "开机自启"; GroupDescription: "附加任务:"; Flags: unchecked

[Files]
Source: "..\dist\bin\*"; DestDir: "{app}\bin"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "..\dist\config\*"; DestDir: "{app}\config"; Flags: ignoreversion recursesubdirs createallsubdirs skipifsourcedoesntexist

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\bin\{#MyAppExeName}"
Name: "{group}\{cm:UninstallProgram,{#MyAppName}}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\bin\{#MyAppExeName}"; Tasks: desktopicon

[Run]
Filename: "{app}\bin\{#MyAppBackendName}"; Description: "启动后台服务"; Flags: nowait postinstall skipifsilent

[Registry]
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "{#MyAppName}"; ValueData: """{app}\bin\{#MyAppExeName}"" --autostart"; Tasks: autostart

[UninstallRun]
Filename: "taskkill"; Parameters: "/F /IM {#MyAppExeName}"; Flags: runhidden
Filename: "taskkill"; Parameters: "/F /IM {#MyAppBackendName}"; Flags: runhidden
