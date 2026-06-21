#pragma once

#include <QObject>
#include <QString>

// TextInjector 把文本注入到当前前台窗口的焦点输入框。
//
// 技术方案（参考 ai-voice-tool）：剪贴板中转 + 模拟 Ctrl+V。
//   1. GetGUIThreadInfo 取前台线程的焦点窗口 hwndFocus
//   2. GetClassName 判断焦点控件是否为可编辑控件（Edit/RichEdit/IME 等）
//   3. 备份当前剪贴板 → 写入待注入文本（CF_UNICODETEXT，支持中文）
//   4. SendInput 模拟 Ctrl+V
//   5. 异步（QTimer 延迟 300ms）恢复原剪贴板
//
// inject(text) 返回 true 表示注入成功，false 表示无可用焦点输入框
// （调用方据此降级，如弹气泡）。
class TextInjector : public QObject {
  Q_OBJECT

public:
  explicit TextInjector(QObject *parent = nullptr);

  // 注入文本到当前前台窗口的焦点输入框。
  // 成功返回 true；若无前台窗口或焦点不是可编辑控件，返回 false。
  Q_INVOKABLE bool inject(const QString &text);

private:
  // 取前台线程的焦点控件 hwnd；失败返回 nullptr。
  static void *focusHwnd();
  // 判断 hwnd 是否为可接收文本输入的控件（Edit/RichEdit/IME/通用）。
  static bool isEditableControl(void *hwnd);
};
