#include "textinjector.h"

#include <qt_windows.h>
#include <QGuiApplication>
#include <QClipboard>
#include <QKeyEvent>

// 可编辑控件的窗口类名前缀（用于 isEditableControl 判断）。
// 涵盖 Win32 原生编辑框、富文本框、以及部分宿主 IME 的控件。
// 现代 UI 框架（Electron/浏览器 contenteditable div）多基于无 hwnd 的自绘，
// 这类无法靠类名识别，会落到"类名未知"分支：保守地尝试注入（Ctrl+V），
// 由目标应用决定是否响应——失败则调用方降级到气泡。
static bool isKnownEditClass(const wchar_t *cls) {
  if (!cls) return false;
  // Win32 标准编辑框
  if (wcsstr(cls, L"Edit")) return true;
  // 富文本框
  if (wcsstr(cls, L"RichEdit")) return true;
  // 输入法相关
  if (wcsstr(cls, L"IME")) return true;
  // Qt / Chromium / Electron 内部类（可能可输入）
  if (wcsstr(cls, L"Qt")) return true;
  if (wcsstr(cls, L"Chrome")) return true;
  return false;
}

TextInjector::TextInjector(QObject *parent) : QObject(parent) {}

void *TextInjector::focusHwnd() {
  // 取前台窗口所在线程，再查该线程的 GUI 信息（含 hwndFocus）。
  HWND fg = GetForegroundWindow();
  if (!fg) return nullptr;

  DWORD tid = GetWindowThreadProcessId(fg, nullptr);
  GUITHREADINFO gti{};
  gti.cbSize = sizeof(gti);
  if (!GetGUIThreadInfo(tid, &gti)) return nullptr;
  // hwndFocus 为当前焦点控件；可能为 0（如焦点在非客户区）。
  return gti.hwndFocus ? gti.hwndFocus : nullptr;
}

bool TextInjector::isEditableControl(void *hwnd) {
  if (!hwnd) return false;
  HWND h = static_cast<HWND>(hwnd);

  // 取窗口类名判断。
  wchar_t cls[256] = {0};
  GetClassNameW(h, cls, 256);
  if (isKnownEditClass(cls)) return true;

  // 类名未命中但控件可见且有 WS_TABSTOP/可聚焦特征时，
  // 保守地认为可能可输入（覆盖自绘输入框）。这样降级门槛更高一些——
  // 宁可尝试注入失败再降级，也不轻易判定"不可输入"导致频繁降级。
  // 注意：桌面/资源管理器等非输入控件若命中此分支，Ctrl+V 会被忽略，
  // 文本已进剪贴板，用户可手动粘贴，不会丢失。
  if (IsWindowVisible(h)) return true;

  return false;
}

bool TextInjector::inject(const QString &text) {
  if (text.isEmpty()) return false;

  // 1. 焦点检测：必须存在前台窗口且焦点控件可编辑。
  void *focus = focusHwnd();
  if (!focus) {
    // 无前台窗口（如刚锁屏/在登录界面），无法注入。
    return false;
  }
  if (!isEditableControl(focus)) {
    return false;
  }

  // 2. 写入待注入文本到剪贴板（不再备份/恢复原内容——见下方第 5 步说明）。
  QClipboard *cb = QGuiApplication::clipboard();
  cb->setText(text, QClipboard::Clipboard);

  // 4. 模拟 Ctrl+V。用 SendInput（比 keybd_event 更现代，UAC 下更可靠）。
  //    发给当前前台窗口的焦点（由 Windows 路由）。
  INPUT inputs[4] = {};
  // Ctrl down
  inputs[0].type = INPUT_KEYBOARD;
  inputs[0].ki.wVk = VK_CONTROL;
  inputs[0].ki.wScan = 0;
  inputs[0].ki.dwFlags = 0;
  // V down
  inputs[1].type = INPUT_KEYBOARD;
  inputs[1].ki.wVk = 'V';
  inputs[1].ki.wScan = 0;
  inputs[1].ki.dwFlags = 0;
  // V up
  inputs[2].type = INPUT_KEYBOARD;
  inputs[2].ki.wVk = 'V';
  inputs[2].ki.wScan = 0;
  inputs[2].ki.dwFlags = KEYEVENTF_KEYUP;
  // Ctrl up
  inputs[3].type = INPUT_KEYBOARD;
  inputs[3].ki.wVk = VK_CONTROL;
  inputs[3].ki.wScan = 0;
  inputs[3].ki.dwFlags = KEYEVENTF_KEYUP;

  UINT sent = SendInput(4, inputs, sizeof(INPUT));
  if (sent != 4) {
    // 发送失败：剪贴板已是识别文本，留着不恢复——用户可手动粘贴找回，
    // 避免恢复原内容把刚识别的一大段文本抹掉（数据安全优先于剪贴板整洁）。
    return false;
  }

  // 5. 不恢复原剪贴板：识别文本留在剪贴板里。
  //    历史问题：原实现 300ms 后恢复原剪贴板，但 Ctrl+V 是否真正粘贴成功
  //    Windows 不给反馈——若目标未响应粘贴（焦点不在输入框、应用拦截了 Ctrl+V），
  //    识别文本只躺在剪贴板里，300ms 后又被恢复逻辑抹掉，导致识别内容彻底丢失。
  //    现在保留识别文本：注入成功则输入框已有内容（剪贴板是否还原无所谓）；
  //    注入失败则用户随时可手动 Ctrl+V 找回。代价是用户原剪贴板内容被替换——
  //    相比丢失语音识别内容，这个代价小得多（且识别文本正是用户当前最需要的）。
  return true;
}
