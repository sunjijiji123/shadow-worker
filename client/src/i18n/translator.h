#pragma once

#include <QObject>
#include <QQmlEngine>
#include <QTranslator>
#include <QString>

// Translator: runtime language switcher for the QML UI.
//
// - Source strings in .qml are English wrapped in qsTr().
// - "en"  : remove translator -> source English shows through.
// - "zh_CN": load :/i18n/shadowworker_zh_CN.qm -> Chinese.
//
// Switch is LIVE (no restart): after install/removeTranslator we call
// QQmlEngine::retranslate() which re-evaluates every qsTr() binding.
//
// Preference is persisted in QSettings (HKCU\Software\ShadowWorker) under
// "ui/language". First launch defaults to "zh_CN".
//
// Exposed to QML as a context property "translator" (see main.cpp).
class Translator : public QObject {
  Q_OBJECT
  QML_ELEMENT
  // Current language code ("zh_CN" | "en"). QML binds menu check marks to it.
  Q_PROPERTY(QString currentLanguage READ currentLanguage
             NOTIFY currentLanguageChanged)

 public:
  explicit Translator(QObject *parent = nullptr);

  // Bind the QML engine so setLanguage can trigger retranslate().
  // Must be called once after QQmlApplicationEngine is constructed.
  void setEngine(QQmlEngine *engine);

  QString currentLanguage() const { return m_current; }

  // Switch language at runtime. code: "zh_CN" | "en".
  // Persists to QSettings and calls engine->retranslate().
  Q_INVOKABLE void setLanguage(const QString &code);

 signals:
  void currentLanguageChanged();

 private:
  // Install the .qm for m_current (called from ctor and setLanguage).
  // For "en" this removes any installed translator.
  void applyLanguage();

  QQmlEngine *m_engine = nullptr;
  QTranslator m_q;       // the currently installed translator (empty if "en")
  bool m_installed = false;
  QString m_current = QStringLiteral("zh_CN");
};
