// Shadow Worker Qt 客户端入口。

#ifdef _WIN32
#include <windows.h>
#endif

#include <QAbstractGrpcChannel>
#include <QApplication>
#include <QDir>
#include <QFileInfo>
#include <QGrpcHttp2Channel>
#include <QIcon>
#include <QLoggingCategory>
#include <QQmlApplicationEngine>
#include <QQmlContext>
#include <QFile>
#include <QMessageLogContext>
#include <QQuickStyle>
#include <QStandardPaths>
#include <QTextStream>
#include <QDateTime>

#include "asr/asrclient.h"
#include "asr/voiceclient.h"
#include "asr/collectionclient.h"
#include "audio/audiorecorder.h"
#include "hotkey/globalhotkey.h"
#include "ui/traycontroller.h"
#include "utils/autostart.h"
#include "utils/backendlauncher.h"
#include "utils/singleinstance.h"
#include "i18n/translator.h"
#include "viewmodels/overview_vm.h"
#include "viewmodels/settings_vm.h"
#include "viewmodels/timeline_vm.h"
#include "viewmodels/update_vm.h"
#include "viewmodels/whitelist_vm.h"
#include "window/windowpicker.h"
#include "window/windowhelper.h"
#include "window/screenshotcontroller.h"
#include "window/textinjector.h"
#include "audio/audiodevicemanager.h"

#include <chrono>
#include <ctime>
#include <fstream>
#include <iomanip>
#include <iostream>

static std::ofstream g_log;

// client 日志目录：与后端日志统一放在 %APPDATA%/shadow-worker/logs/。
// 文件名 client-YYYY-MM-DD.log，按天滚动（追加模式）。
// 修复坑 #34：GUI 程序无控制台，qDebug/qWarning 全部丢弃，QML 运行时错误
// 无从排查。此处用 qInstallMessageHandler 拦截所有 Qt 消息（含 QML 绑定错误、
// 类型错误等），统一落盘。与后端 logging.go 范式一致（同一目录、按天文件）。
static QString clientLogPath() {
  // 直接读 APPDATA 环境变量（与后端 Go 的 os.UserConfigDir() 完全一致），
  // 不用 QStandardPaths——它受 OrganizationName/ApplicationName 影响，
  // 而 logMsg 在 setOrganizationName 之前就被调用了，路径可能拼错。
  QString base;
#ifdef _WIN32
  wchar_t buf[MAX_PATH];
  DWORD len = GetEnvironmentVariableW(L"APPDATA", buf, MAX_PATH);
  if (len > 0 && len < MAX_PATH) {
    base = QString::fromWCharArray(buf);
  }
#endif
  if (base.isEmpty()) {
    // 非Windows或读不到APPDATA时，回退到 QStandardPaths。
    base = QStandardPaths::writableLocation(QStandardPaths::GenericConfigLocation);
  }
  QString dir = base + "/shadow-worker/logs";
  QDir().mkpath(dir);
  return dir + "/client-" + QDate::currentDate().toString("yyyy-MM-dd") + ".log";
}

static void logMsg(const std::string &msg) {
  if (!g_log.is_open()) {
    g_log.open(clientLogPath().toStdString(), std::ios::app);
  }
  // 加时间戳前缀，方便对照后端日志。
  auto now = std::chrono::system_clock::now();
  auto t = std::chrono::system_clock::to_time_t(now);
  auto ms = std::chrono::duration_cast<std::chrono::milliseconds>(
                now.time_since_epoch()) % 1000;
  std::tm tm{};
#ifdef _WIN32
  localtime_s(&tm, &t);
#else
  localtime_r(&t, &tm);
#endif
  char ts[24];
  std::strftime(ts, sizeof(ts), "%H:%M:%S", &tm);
  g_log << ts << "." << std::setfill('0') << std::setw(3) << ms.count()
        << " " << msg << std::endl;
  g_log.flush();
}

// qtMessageHandler 拦截所有 qDebug/qWarning/qCritical/qFatal 输出（含 QML 引擎
// 产生的绑定错误、类型错误等），统一写入客户端日志文件。
// 格式：[level] message (file:line)
static void qtMessageHandler(QtMsgType type, const QMessageLogContext &ctx,
                             const QString &msg) {
  const char *levelStr = "DEBUG";
  switch (type) {
    case QtWarningMsg:     levelStr = "WARN";  break;
    case QtCriticalMsg:    levelStr = "ERROR"; break;
    case QtFatalMsg:       levelStr = "FATAL"; break;
    case QtInfoMsg:        levelStr = "INFO";  break;
    default:               levelStr = "DEBUG"; break;
  }
  std::string line = std::string("[") + levelStr + "] " + msg.toStdString();
  // 附带 file:line（QML 错误的关键定位信息），过滤空的。
  if (ctx.file && ctx.file[0] != '\0') {
    line += " (" + std::string(ctx.file);
    if (ctx.line > 0) line += ":" + std::to_string(ctx.line);
    line += ")";
  }
  logMsg(line);
  // Fatal 消息 Qt 默认会 abort，确保日志刷盘（logMsg 已 flush）。
}

int main(int argc, char *argv[]) {
  logMsg("[main] start");

  try {
    QGuiApplication::setHighDpiScaleFactorRoundingPolicy(
        Qt::HighDpiScaleFactorRoundingPolicy::PassThrough);

    logMsg("[main] before QApplication");
    QApplication app(argc, argv);
    logMsg("[main] after QApplication");

    // 装 Qt 消息处理器：拦截所有 qDebug/qWarning/qCritical（含 QML 运行时错误），
    // 统一写入客户端日志文件。修复坑 #34（GUI 程序无控制台，Qt 消息全部丢弃）。
    qInstallMessageHandler(qtMessageHandler);
    logMsg("[main] Qt message handler installed (client log -> logs/client-*.log)");

    QApplication::setApplicationName("Shadow Worker");
    QApplication::setOrganizationName("ShadowWorker");
    // Product icon: loaded from qrc (assets/app.ico compiled into qrc via
    // qt_add_qml_module RESOURCES). Affects window titlebar + taskbar.
    // The same .ico is also embedded via app.rc for Windows Explorer/shortcuts.
    QApplication::setWindowIcon(
        QIcon(QStringLiteral(":/qt/qml/ShadowWorker/assets/app.ico")));
    // Keep the process alive when the main window is hidden to the tray.
    // Quit only happens via the tray menu's Quit item.
    QApplication::setQuitOnLastWindowClosed(false);

    // --- Single instance check ---
    // Named Mutex: second instance detects mutex already exists, activates
    // the running instance's window, and exits.
    if (!SingleInstance::tryLock()) {
      logMsg("[main] another instance running, activating it and quit");
      SingleInstance::activateExistingInstance();
      return 0;
    }

    // --- Launch backend ---
    // startDetached so the backend survives independently; we track its PID
    // and kill it on aboutToQuit. If exe not found, run in degraded mode
    // (gRPC calls will fail until backend is manually started).
    BackendLauncher backend;
    if (!backend.start()) {
      logMsg("[main] backend exe not found or launch failed, degraded mode");
    }
    QObject::connect(&app, &QApplication::aboutToQuit, &backend,
                     &BackendLauncher::stop);

    QLoggingCategory::setFilterRules(
        "qt.qml.binding.removal.warning=true\nqml=true");
    QQuickStyle::setStyle("Basic");

    logMsg("[main] creating channel");
    auto channel =
        std::make_shared<QGrpcHttp2Channel>(QUrl("http://127.0.0.1:50051"));

    logMsg("[main] creating objects");
    OverviewViewModel overviewVm;
    overviewVm.setChannel(channel);
    WhitelistViewModel whitelistVm;
    TimelineViewModel timelineVm;
    timelineVm.setChannel(channel);
    SettingsViewModel settingsVm;
    settingsVm.setChannel(channel);
    UpdateViewModel updateVm;
    updateVm.setChannel(channel);
    // 下载完成并拉起安装包后，请求退出客户端，让 Inno Setup 能覆盖 exe 文件
    // （否则文件被锁，PrepareToInstall 的 taskkill 走不到复制阶段）。
    QObject::connect(&updateVm, &UpdateViewModel::requestQuit,
                     &app, &QApplication::quit);
    AutostartManager autostartManager;
    const bool autostartMode =
        QCoreApplication::arguments().contains(QLatin1String("--autostart"));
    WindowPicker picker;
    WindowHelper windowHelper;
    ScreenshotController screenshotController;
    TextInjector textInjector;
    AudioRecorder audioRecorder;
    AudioDeviceManager audioDeviceManager;
    GlobalHotkey globalHotkey;
    AsrClient asrClient;
    VoiceClient voiceClient;
    voiceClient.setChannel(channel);
    CollectionClient collectionClient;
    collectionClient.setChannel(channel);
    TrayController trayController;
    Translator translator;
    // Translator 构造时已 installTranslator（读 QSettings 默认 zh_CN）。
    // 但 TrayController(L125) 在 Translator(L126) 之前构造，其菜单 tr()
    // 执行时翻译器尚未安装，文字定格英文。这里显式重翻译一次。
    trayController.retranslateUi();
    // 运行时切换语言时，QML 有 engine->retranslate() 自动刷新，但 C++
    // QAction 没有，需手动重翻译托盘菜单。
    QObject::connect(&translator, &Translator::currentLanguageChanged,
                     &trayController, &TrayController::retranslateUi);

    logMsg("[main] creating engine");
    QQmlApplicationEngine engine;

    // Bind the QML engine to the translator so live language switches can
    // trigger retranslate(). Must be called BEFORE engine.load() so that
    // the initial language (read from QSettings in Translator's ctor) is
    // already applied when QML is first evaluated.
    translator.setEngine(&engine);

    // Read version from VERSION file (exe's directory).
    // Falls back to "unknown" if missing.
    {
      const QString exeDir =
          QFileInfo(QCoreApplication::applicationFilePath()).absolutePath();
      QFile vf(exeDir + QDir::separator() + QStringLiteral("VERSION"));
      QString ver = QStringLiteral("unknown");
      if (vf.open(QIODevice::ReadOnly | QIODevice::Text)) {
        ver = QString::fromUtf8(vf.readAll()).trimmed();
      }
      engine.rootContext()->setContextProperty("appVersion", ver);
      updateVm.setCurrentVersion(ver);
    }

    engine.rootContext()->setContextProperty("overviewVm", &overviewVm);
    engine.rootContext()->setContextProperty("whitelistVm", &whitelistVm);
    engine.rootContext()->setContextProperty("timelineVm", &timelineVm);
    engine.rootContext()->setContextProperty("settingsVm", &settingsVm);
    engine.rootContext()->setContextProperty("updateVm", &updateVm);
    engine.rootContext()->setContextProperty("autostartManager",
                                             &autostartManager);
    engine.rootContext()->setContextProperty("autostartMode", autostartMode);
    engine.rootContext()->setContextProperty("windowPicker", &picker);
    engine.rootContext()->setContextProperty("windowHelper", &windowHelper);
    engine.rootContext()->setContextProperty("screenshotController",
                                             &screenshotController);
    engine.rootContext()->setContextProperty("textInjector", &textInjector);
    engine.rootContext()->setContextProperty("audioRecorder", &audioRecorder);
    engine.rootContext()->setContextProperty("audioDeviceManager", &audioDeviceManager);
    engine.rootContext()->setContextProperty("globalHotkey", &globalHotkey);
    engine.rootContext()->setContextProperty("asrClient", &asrClient);
    engine.rootContext()->setContextProperty("voiceClient", &voiceClient);
    engine.rootContext()->setContextProperty("collectionClient",
                                             &collectionClient);
    engine.rootContext()->setContextProperty("trayController", &trayController);
    engine.rootContext()->setContextProperty("translator", &translator);

    // Resolve the MCP server executable path for the MCP config snippet.
    // 用 resolveMcpExePath（指向 shadow-worker-mcp.exe，独立于主后端）而非
    // resolveExePath（主后端 shadow-worker.exe）：MCP 拆成独立 exe 后，升级主程序
    // 不会被 agent 持有的 MCP 子进程锁文件阻断（见 AGENTS.md 坑 50）。
    // Expose both the chosen path and a "ready" flag so the System page can
    // show an accurate status light (MCP is usable iff the exe exists).
    // mcpShortPath：exe 的 8.3 短路径（仅 Windows，如 C:\PROGRA~2\...\shadow-worker-mcp.exe）。
    // 用于 work buddy/TRAE 配置（裸路径 command，不能带引号，短路径避开空格截断）。
    // 转换失败（API 报错 / 非 Windows）为空串，前端 workbuddy tab 回退到长路径+提示。
    QString mcpExePath = BackendLauncher::resolveMcpExePath();
    QString mcpShortPath = BackendLauncher::resolveMcpShortPath();
    engine.rootContext()->setContextProperty("mcpExePath", mcpExePath);
    engine.rootContext()->setContextProperty("mcpShortPath", mcpShortPath);
    engine.rootContext()->setContextProperty("mcpReady", !mcpExePath.isEmpty());

    // Tray "Quit" -> actually quit the app (C++-level guarantee).
    QObject::connect(&trayController, &TrayController::quitRequested, &app,
                     &QApplication::quit);

    const QUrl url(u"qrc:/qt/qml/ShadowWorker/qml/main.qml"_qs);
    QObject::connect(
        &engine, &QQmlApplicationEngine::objectCreationFailed, &app,
        []() {
          logMsg("[FATAL] QML object creation failed");
          QCoreApplication::exit(-1);
        },
        Qt::QueuedConnection);

    logMsg("[main] loading qml");
    QObject::connect(&engine, &QQmlApplicationEngine::warnings,
                     [](const QList<QQmlError> &warnings) {
                       for (const auto &err : warnings) {
                         logMsg("[QML WARNING] " +
                                err.toString().toStdString());
                       }
                     });
    engine.load(url);

    if (engine.rootObjects().isEmpty()) {
      logMsg("[FATAL] engine.rootObjects() empty after load");
      return -1;
    }

    logMsg("[main] exec");
    return app.exec();
  } catch (const std::exception &e) {
    logMsg(std::string("[FATAL] exception: ") + e.what());
    return -1;
  } catch (...) {
    logMsg("[FATAL] unknown exception");
    return -1;
  }
}
