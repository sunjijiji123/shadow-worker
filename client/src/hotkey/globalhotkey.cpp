// GlobalHotkey 实现 (Windows)
//
// 两种模式：
//   press: RegisterHotKey + WM_HOTKEY → emit activated（toggle 语义）
//   hold:  RegisterHotKey 检测按下 → emit pressed；
//          QTimer 轮询 GetAsyncKeyState 检测松开 → emit released。
//
// 历史教训：早期版本用 WH_KEYBOARD_LL 低级键盘钩子检测松开，但该钩子的
// 回调跑在系统键盘链路里。如果回调里同步执行慢操作（如 emit → gRPC 调用），
// 会阻塞整个系统的键盘输入，导致电脑卡顿。改为 QTimer 轮询后，所有处理
// 都在 Qt 主线程事件循环内完成，绝不阻塞系统键盘。

#include "globalhotkey.h"

#include <QApplication>
#include <QDebug>
#include <QGuiApplication>

#include <windows.h>

// ---- GlobalHotkey ----

GlobalHotkey::GlobalHotkey(QObject *parent) : QObject(parent) {
  QGuiApplication::instance()->installNativeEventFilter(this);
  m_installed = true;

  // hold 模式轮询定时器：50ms 一次检测松开（约 20fps，足够灵敏）。
  m_holdPollTimer.setInterval(50);
  connect(&m_holdPollTimer, &QTimer::timeout, this,
          &GlobalHotkey::onHoldPollTick);
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
  m_registrations[id] = {id, QStringLiteral("record"), QStringLiteral("press"),
                         MOD_CONTROL | MOD_SHIFT, vk};
  return true;
}

bool GlobalHotkey::registerShortcut(const QString &shortcut,
                                    const QString &name,
                                    const QString &mode) {
  if (shortcut.trimmed().isEmpty())
    return false;

  uint modifiers = 0;
  int vk = 0;
  if (!parseShortcut(shortcut, modifiers, vk))
    return false;

  QString actualMode = mode.isEmpty() ? QStringLiteral("press") : mode.toLower();
  int id = m_nextId++;

  if (!RegisterHotKey(nullptr, id, modifiers, static_cast<UINT>(vk))) {
    qWarning() << "RegisterHotKey failed for" << shortcut
               << "error:" << GetLastError();
    return false;
  }

  m_registrations[id] = {id, name.isEmpty() ? shortcut : name, actualMode,
                         modifiers, vk};

  // hold 模式：记录 vk，但不立即启动轮询。
  // 轮询必须在 WM_HOTKEY（真正按下）时才启动——否则注册时键是松开的，
  // 第一次 tick 就会误判"已松开"并停止轮询，导致真正按下后检测不到松开。
  if (actualMode == QStringLiteral("hold")) {
    m_holdVk = vk;
    m_holdName = name.isEmpty() ? shortcut : name;
  }

  return true;
}

void GlobalHotkey::unregisterAll() {
  stopHoldPolling();
  m_holdVk = 0;   // 彻底注销：清掉 hold 配置
  for (auto it = m_registrations.begin(); it != m_registrations.end(); ++it) {
    UnregisterHotKey(nullptr, it.key());
  }
  m_registrations.clear();
}

void GlobalHotkey::stopHoldPolling() {
  m_holdPollTimer.stop();
  // 注意：不清零 m_holdVk！它是注册时设的"配置"，不是运行状态。
  // 清零会导致第二次按下时 tick 因 m_holdVk==0 立即退出，检测不到松开。
  // m_holdVk 只在 unregisterAll 时清零。
}

void GlobalHotkey::onHoldPollTick() {
  if (m_holdVk == 0) {
    // 没有注册 hold 热键，不应到达这里；保险起见停掉定时器。
    m_holdPollTimer.stop();
    return;
  }
  // GetAsyncKeyState 最高位为 1 = 当前按下；为 0 = 已松开
  SHORT state = GetAsyncKeyState(m_holdVk);
  bool down = (state & 0x8000) != 0;
  if (!down) {
    // 检测到松开：emit released 并停止轮询
    QString name = m_holdName;
    stopHoldPolling();
    emit released(name);
  }
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
      const Reg &reg = it.value();

      if (reg.mode == QStringLiteral("hold")) {
        // hold 模式：按下 = 开始录音。此时键确实处于按下状态，
        // 启动轮询检测松开。
        // 注意：RegisterHotKey 在某些配置下按住期间会重复触发 WM_HOTKEY
        // （约每 30ms 一次）。如果每次都 m_holdPollTimer.start()，50ms 的
        // 定时器永远到不了 timeout（被不断重置），轮询 tick 永不执行，
        // 松开就检测不到。所以只在轮询未运行时启动一次。
        if (!m_holdPollTimer.isActive()) {
          m_holdPollTimer.start();
        }
        emit pressed(reg.name);
      } else {
        // press 模式：toggle
        emit activatedWithName(reg.name);
        emit activated();
      }
      return true;
    }
  }
  return false;
}
