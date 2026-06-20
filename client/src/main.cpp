// Shadow Worker Qt 客户端入口。

#include <QAbstractGrpcChannel>
#include <QApplication>
#include <QGrpcHttp2Channel>
#include <QLoggingCategory>
#include <QQmlApplicationEngine>
#include <QQmlContext>
#include <QQuickStyle>

#include "asr/asrclient.h"
#include "asr/voiceclient.h"
#include "audio/audiorecorder.h"
#include "hotkey/globalhotkey.h"
#include "ui/traycontroller.h"
#include "utils/autostart.h"
#include "viewmodels/overview_vm.h"
#include "viewmodels/settings_vm.h"
#include "viewmodels/timeline_vm.h"
#include "viewmodels/whitelist_vm.h"
#include "window/windowpicker.h"
#include "window/windowhelper.h"
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
    // Keep the process alive when the main window is hidden to the tray.
    // Quit only happens via the tray menu's Quit item.
    QApplication::setQuitOnLastWindowClosed(false);

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
    AudioRecorder audioRecorder;
    AudioDeviceManager audioDeviceManager;
    GlobalHotkey globalHotkey;
    AsrClient asrClient;
    VoiceClient voiceClient;
    voiceClient.setChannel(channel);
    TrayController trayController;

    logMsg("[main] creating engine");
    QQmlApplicationEngine engine;

    engine.rootContext()->setContextProperty("overviewVm", &overviewVm);
    engine.rootContext()->setContextProperty("whitelistVm", &whitelistVm);
    engine.rootContext()->setContextProperty("timelineVm", &timelineVm);
    engine.rootContext()->setContextProperty("settingsVm", &settingsVm);
    engine.rootContext()->setContextProperty("autostartManager",
                                             &autostartManager);
    engine.rootContext()->setContextProperty("autostartMode", autostartMode);
    engine.rootContext()->setContextProperty("windowPicker", &picker);
    engine.rootContext()->setContextProperty("windowHelper", &windowHelper);
    engine.rootContext()->setContextProperty("audioRecorder", &audioRecorder);
    engine.rootContext()->setContextProperty("audioDeviceManager", &audioDeviceManager);
    engine.rootContext()->setContextProperty("globalHotkey", &globalHotkey);
    engine.rootContext()->setContextProperty("asrClient", &asrClient);
    engine.rootContext()->setContextProperty("voiceClient", &voiceClient);
    engine.rootContext()->setContextProperty("trayController", &trayController);

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
