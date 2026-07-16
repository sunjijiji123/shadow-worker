#pragma once

#include <QImage>
#include <QQuickAsyncImageProvider>
#include <QQuickTextureFactory>
#include <QSize>
#include <QGrpcHttp2Channel>

#include <memory>

#include "whitelist.qpb.h"
#include "whitelist_client.grpc.qpb.h"

// WindowThumbnailProvider 为 QML 的 "image://winthumb/<hwnd>" 提供窗口缩略图。
//
// 用途：「添加采集应用」对话框的截图网格里，每张卡片用 Image { source:
// "image://winthumb/" + hwnd } 懒加载该窗口的实时截图（320×180 PNG）。
//
// 【必须用异步 provider】早期实现继承 QQuickImageProvider(Image) 并在
// requestImage 里用 QEventLoop 同步阻塞等 gRPC 回调。但 Image 类型的 provider
// 跑在 QML image loader 线程，该线程没有独立事件循环在驱动 QGrpcHttp2Channel
// 的网络回调 → loop.exec() 永远等不到 finished 信号 → 卡死 image loader 线程
// → 整个 QML 渲染管线被拖垮（症状：列表区空白、Repeater delegate 不渲染，
// 但 root.windows 数据已到位、selectedPath 被点击设上 → "无可见窗口但已选择"
// 的矛盾状态）。
//
// 改用 QQuickAsyncImageProvider：requestImageResponse 立即返回一个 response
// 对象，在其中发起 gRPC 调用并持有 reply；gRPC 的 finished 回调在主线程（通道
// 所在线程）到达时解析 PNG 并 emit finished()，Qt 图像加载框架据此异步完成。
// 不阻塞任何线程。
//
// 失败降级（窗口已关/hwnd 失效/gRPC 错）：返回首字母占位图。
class WindowThumbnailProvider : public QQuickAsyncImageProvider {
public:
  WindowThumbnailProvider();

  // id 格式：<hwnd>[@<appName>]。返回的 response 持有 gRPC reply，完成后
  // emit finished() 触发 Qt 异步图像加载完成。
  QQuickImageResponse *requestImageResponse(const QString &id,
                                            const QSize &requestedSize) override;
};
