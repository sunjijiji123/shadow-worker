#pragma once

#include <QObject>
#include <QString>
#include <QTimer>

// WindowPicker 提供"选窗口"能力。
// 当前为简化实现：调用 pick() 后等待几秒，取当前前台窗口。
// 注意：不声明 QML_ELEMENT。它只作为 context property "windowPicker" 使用；
// 若加 QML_ELEMENT，注册的类型名 WindowPicker 会与 context property 名冲突，
// 导致 QML 子组件里裸引用 windowPicker 时解析为类型而非注入实例（返回 null）。
class WindowPicker : public QObject {
  Q_OBJECT
  Q_PROPERTY(bool picking READ picking NOTIFY pickingChanged)

public:
  explicit WindowPicker(QObject *parent = nullptr);

  bool picking() const { return m_picking; }

  Q_INVOKABLE void pick();

signals:
  void pickingChanged();
  void picked(const QString &path, const QString &name, const QString &title);
  void cancelled();

private:
  void finishPick();
  bool m_picking = false;
  QTimer *m_timer = nullptr;
};
