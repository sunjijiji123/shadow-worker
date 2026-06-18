// GlobalHotkey 实现 (Windows RegisterHotKey)

#include "globalhotkey.h"

#include <QDebug>
#include <QGuiApplication>

#include <windows.h>

GlobalHotkey::GlobalHotkey(QObject *parent) : QObject(parent) {
  QGuiApplication::instance()->installNativeEventFilter(this);
  m_installed = true;
}

GlobalHotkey::~GlobalHotkey() {
  unregisterAll();
  if (m_installed && QGuiApplication::instance()) {
    QGuiApplication::instance()->removeNativeEventFilter(this);
  }
}

bool GlobalHotkey::registerHotkey(int vk) {
  if (vk <= 0)
    return false;
  int id = m_nextId++;
  if (!RegisterHotKey(nullptr, id, MOD_CONTROL | MOD_SHIFT,
                      static_cast<UINT>(vk)))
    return false;
  m_registrations[id] = {id, QStringLiteral("record")};
  return true;
}

bool GlobalHotkey::registerShortcut(const QString &shortcut,
                                    const QString &name) {
  if (shortcut.trimmed().isEmpty())
    return false;

  uint modifiers = 0;
  int vk = 0;
  if (!parseShortcut(shortcut, modifiers, vk))
    return false;

  int id = m_nextId++;
  if (!RegisterHotKey(nullptr, id, modifiers, static_cast<UINT>(vk))) {
    qWarning() << "RegisterHotKey failed for" << shortcut
               << "error:" << GetLastError();
    return false;
  }
  m_registrations[id] = {id, name.isEmpty() ? shortcut : name};
  return true;
}

void GlobalHotkey::unregisterAll() {
  for (auto it = m_registrations.begin(); it != m_registrations.end(); ++it) {
    UnregisterHotKey(nullptr, it.key());
  }
  m_registrations.clear();
}

bool GlobalHotkey::parseShortcut(const QString &shortcut, uint &modifiers,
                                 int &vk) {
  modifiers = 0;
  vk = 0;

  QString name = shortcut.trimmed();
  QStringList parts = name.split('+', Qt::SkipEmptyParts);
  QString keyPart;
  for (const QString &part : parts) {
    QString p = part.trimmed().toLower();
    if (p == "ctrl" || p == "control") {
      modifiers |= MOD_CONTROL;
    } else if (p == "alt") {
      modifiers |= MOD_ALT;
    } else if (p == "shift") {
      modifiers |= MOD_SHIFT;
    } else if (p == "win" || p == "windows" || p == "meta") {
      modifiers |= MOD_WIN;
    } else {
      keyPart = part.trimmed();
    }
  }

  if (keyPart.isEmpty() && parts.size() == 1) {
    keyPart = parts.first().trimmed();
  }
  if (keyPart.isEmpty())
    return false;

  vk = keyToVk(keyPart);
  return vk > 0;
}

int GlobalHotkey::keyToVk(const QString &key) {
  QString k = key.toUpper();

  if (k.startsWith('F') && k.length() <= 3) {
    int n = k.mid(1).toInt();
    if (n >= 1 && n <= 24)
      return VK_F1 + (n - 1);
  }

  static const QMap<QString, int> special{
      {"ESC", VK_ESCAPE},        {"TAB", VK_TAB},       {"SPACE", VK_SPACE},
      {"ENTER", VK_RETURN},      {"RETURN", VK_RETURN}, {"BACKSPACE", VK_BACK},
      {"DELETE", VK_DELETE},     {"DEL", VK_DELETE},    {"INSERT", VK_INSERT},
      {"HOME", VK_HOME},         {"END", VK_END},       {"PGUP", VK_PRIOR},
      {"PGDOWN", VK_NEXT},       {"UP", VK_UP},         {"DOWN", VK_DOWN},
      {"LEFT", VK_LEFT},         {"RIGHT", VK_RIGHT},   {"PRINT", VK_PRINT},
      {"SNAPSHOT", VK_SNAPSHOT}, {"PRTSC", VK_SNAPSHOT}};
  if (special.contains(k))
    return special.value(k);

  if (k.length() == 1) {
    QChar c = k.at(0);
    if (c >= '0' && c <= '9')
      return '0' + (c.unicode() - '0');
    if (c >= 'A' && c <= 'Z')
      return 'A' + (c.unicode() - 'A');
  }

  int vk =
      VkKeyScanEx(key.at(0).toLower().unicode(), GetKeyboardLayout(0)) & 0xFF;
  if (vk > 0)
    return vk;

  return 0;
}

bool GlobalHotkey::nativeEventFilter(const QByteArray &eventType, void *message,
                                     qintptr *result) {
  Q_UNUSED(eventType)
  Q_UNUSED(result)

  MSG *msg = static_cast<MSG *>(message);
  if (msg->message == WM_HOTKEY) {
    int id = static_cast<int>(msg->wParam);
    auto it = m_registrations.find(id);
    if (it != m_registrations.end()) {
      emit activatedWithName(it->name);
      emit activated();
      return true;
    }
  }
  return false;
}
