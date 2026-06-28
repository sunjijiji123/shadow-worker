#include "translator.h"

#include <QCoreApplication>
#include <QSettings>
#include <QLoggingCategory>

Translator::Translator(QObject *parent) : QObject(parent) {
  // Read saved preference; default to Simplified Chinese on first launch.
  QSettings s;
  s.beginGroup(QStringLiteral("ui"));
  m_current = s.value(QStringLiteral("language"),
                      QStringLiteral("zh_CN")).toString();
  s.endGroup();
  // Apply immediately so the very first QML load is already localized.
  // (Engine isn't set yet, but installTranslator works without it;
  //  retranslate() is only needed for live switches after load.)
  applyLanguage();
}

void Translator::setEngine(QQmlEngine *engine) {
  m_engine = engine;
}

void Translator::setLanguage(const QString &code) {
  if (code == m_current) return;
  m_current = code;

  QSettings s;
  s.beginGroup(QStringLiteral("ui"));
  s.setValue(QStringLiteral("language"), code);
  s.endGroup();

  applyLanguage();

  if (m_engine) {
    // Re-evaluate every qsTr() binding in the QML tree. This is the
    // "no restart" mechanism: all bound strings refresh in place.
    m_engine->retranslate();
  }
  emit currentLanguageChanged();
}

void Translator::applyLanguage() {
  // Remove the previously installed translator (if any).
  if (m_installed) {
    QCoreApplication::removeTranslator(&m_q);
    m_installed = false;
  }
  // "en" = source language: no .qm needed, English shows through.
  if (m_current == QStringLiteral("en")) return;
  // Load the compiled .qm from the qrc resource ":/i18n/<lang>.qm".
  const QString qmPath = QStringLiteral(":/i18n/shadowworker_") + m_current +
                         QStringLiteral(".qm");
  if (m_q.load(qmPath)) {
    QCoreApplication::installTranslator(&m_q);
    m_installed = true;
  } else {
    qWarning("Translator: failed to load %s", qPrintable(qmPath));
  }
}
