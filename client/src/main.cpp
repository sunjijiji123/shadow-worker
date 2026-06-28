// Shadow Worker Qt 客户端入口。

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
#include <QQuickStyle>

#include "asr/asrclient.h"
#include "asr/voiceclient.h"
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
#include "viewmodels/whitelist_vm.h"
#include "window/windowpicker.h"
#include "window/windowhelper.h"
#include "window/textinjector.h"
#include "audio/audiodevicemanager.h"

#include <fstream>
#include <iostream>

static std::ofstream g_log;

static void logMsg(const std::string &msg) {
  if (!g_log.is_open()) {
    // write to %TEMP% for portability (old hardcoded path no longer exists)
    const char *tmp = std::getenv("TEMP");
    std::string path = tmp ? tmp : ".";
    path += "\\shadow-worker-client.log";
    g_log.open(path, std::ios::app);
  }
  g_log << msg << std::endl;
  g_log.flush();
}

int main(int argc, char *argv[]) {
  logMsg("[main] start");

  try {
    QGuiApplication::setHighDpiScaleFactorRoundingPolicy(
        Qt::HighDpiScaleFactorRoundingPolicy::PassThrough);

    logMsg("[main] before QApplication");
    QApplication app(argc, argv);
    logMsg("[main] after QApplication");

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
    AutostartManager autostartManager;
    const bool autostartMode =
        QCoreApplication::arguments().contains(QLatin1String("--autostart"));
    WindowPicker picker;
    WindowHelper windowHelper;
    TextInjector textInjector;
    AudioRecorder audioRecorder;
    AudioDeviceManager audioDeviceManager;
    GlobalHotkey globalHotkey;
    AsrClient asrClient;
    VoiceClient voiceClient;
    voiceClient.setChannel(channel);
    TrayController trayController;
    Translator translator;

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
    }

    engine.rootContext()->setContextProperty("overviewVm", &overviewVm);
    engine.rootContext()->setContextProperty("whitelistVm", &whitelistVm);
    engine.rootContext()->setContextProperty("timelineVm", &timelineVm);
    engine.rootContext()->setContextProperty("settingsVm", &settingsVm);
    engine.rootContext()->setContextProperty("autostartManager",
                                             &autostartManager);
    engine.rootContext()->setContextProperty("autostartMode", autostartMode);
    engine.rootContext()->setContextProperty("windowPicker", &picker);
    engine.rootContext()->setContextProperty("windowHelper", &windowHelper);
    engine.rootContext()->setContextProperty("textInjector", &textInjector);
    engine.rootContext()->setContextProperty("audioRecorder", &audioRecorder);
    engine.rootContext()->setContextProperty("audioDeviceManager", &audioDeviceManager);
    engine.rootContext()->setContextProperty("globalHotkey", &globalHotkey);
    engine.rootContext()->setContextProperty("asrClient", &asrClient);
    engine.rootContext()->setContextProperty("voiceClient", &voiceClient);
    engine.rootContext()->setContextProperty("trayController", &trayController);
    engine.rootContext()->setContextProperty("translator", &translator);

    // Resolve the backend executable path for the MCP config snippet.
    // Expose both the chosen path and a "ready" flag so the System page can
    // show an accurate status light (MCP is usable iff the exe exists).
    QString mcpExePath = BackendLauncher::resolveExePath();
    engine.rootContext()->setContextProperty("mcpExePath", mcpExePath);
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
