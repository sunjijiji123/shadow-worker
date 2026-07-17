#include "windowthumbnailprovider.h"

#include <QGrpcCallReply>
#include <QGrpcHttp2Channel>
#include <QGrpcStatus>
#include <QPainter>

// 网格卡片宽约 200px，截图区 16:9 约 112px 高。缩略图本身是 320×180 PNG，
// 但 requestedSize 可能是卡片实际显示尺寸。这里给一个兜底默认尺寸。
static constexpr int kDefaultWidth = 320;
static constexpr int kDefaultHeight = 180;

WindowThumbnailProvider::WindowThumbnailProvider()
    : QQuickAsyncImageProvider() {
  // channel/client 创建移到每个 response 内（见下），因为 provider 本身是无
  // 状态的单例（QQmlEngine 持有），而 gRPC client 需要与 response 生命周期绑定。
  // 共用一个全局 client 也可，但每个 response 独立更简单可靠，HTTP/2 多路复用
  // 下连接开销可忽略。
}

namespace {

// 生成失败降级占位图：灰底 + 居中首字母色块。纯函数，无副作用。
QImage makePlaceholder(const QString &appName, const QSize &size) {
  int w = size.width() > 0 ? size.width() : kDefaultWidth;
  int h = size.height() > 0 ? size.height() : kDefaultHeight;
  QImage img(w, h, QImage::Format_RGB32);

  // 灰底（与 Theme.bg 一致 #18181B），克制不抢眼。
  img.fill(QColor(0x18, 0x18, 0x1B));

  QPainter p(&img);
  p.setRenderHint(QPainter::Antialiasing);

  // 居中首字母色块：other 类别灰 #6B7280。取应用名前 2 字符（去 .exe 后缀）。
  QString clean = appName;
  const int dot = clean.lastIndexOf(QLatin1Char('.'));
  if (dot > 0) clean = clean.left(dot);
  const QString initials = clean.left(2).toUpper();

  const int boxSize = qMin(w, h) / 3;
  const QRect boxRect((w - boxSize) / 2, (h - boxSize) / 2, boxSize, boxSize);
  p.setBrush(QColor(0x6B, 0x72, 0x80));
  p.setPen(Qt::NoPen);
  p.drawRoundedRect(boxRect, boxSize / 5, boxSize / 5);

  QFont font;
  font.setPixelSize(boxSize / 2);
  font.setBold(true);
  p.setFont(font);
  p.setPen(QColor(0xFF, 0xFF, 0xFF));
  p.drawText(boxRect, Qt::AlignCenter, initials);

  return img;
}

// ThumbnailResponse 持有一次缩略图 gRPC 调用，完成后 emit finished()。
// 【关键】它是个 QObject，gRPC 的 finished 回调在其所属线程（通道线程，通常
// 主线程）触发，与 requestImageResponse 的调用线程解耦——没有任何线程被阻塞。
class ThumbnailResponse : public QQuickImageResponse {
public:
  explicit ThumbnailResponse(const QString &id, const QSize &requestedSize)
      : m_requestedSize(requestedSize) {
    // 解析 id：<hwnd>[@<appName>]
    int atIdx = id.indexOf(QLatin1Char('@'));
    if (atIdx > 0) {
      m_hwnd = QStringView(id.left(atIdx)).toLongLong();
      m_appName = id.mid(atIdx + 1);
    } else {
      m_hwnd = id.toLongLong();
    }

    if (m_hwnd == 0) {
      // 无效 hwnd，直接给占位图。
      finishWithPlaceholder();
      return;
    }

    // 建独立 gRPC channel + client，绑定到本 response 的线程上下文。
    m_channel = std::make_shared<QGrpcHttp2Channel>(
        QUrl("http://127.0.0.1:50051"));
    m_client = std::make_unique<shadowworker::WhitelistService::Client>();
    m_client->attachChannel(m_channel);

    shadowworker::ThumbnailRequest req;
    req.setHwnd(m_hwnd);
    m_reply = m_client->GetWindowThumbnail(req);

    // gRPC finished 回调在通道线程触发（主线程），不阻塞 image loader 线程。
    QObject::connect(
        m_reply.get(), &QGrpcCallReply::finished, this,
        [this](const QGrpcStatus &status) {
          if (status.isOk()) {
            auto data = m_reply->read<shadowworker::ThumbnailData>();
            if (data && data->png().size() > 0) {
              m_image.loadFromData(data->png(), "PNG");
            }
          }
          // status 非 ok 或 png 为空 → m_image 仍为空，用占位图。
          if (m_image.isNull()) {
            m_image = makePlaceholder(m_appName, m_requestedSize);
          }
          scaleIfNeeded();
          emit finished();
        });
  }

  QQuickTextureFactory *textureFactory() const override {
    return QQuickTextureFactory::textureFactoryForImage(m_image);
  }

private:
  void finishWithPlaceholder() {
    m_image = makePlaceholder(m_appName, m_requestedSize);
    scaleIfNeeded();
    emit finished();
  }

  void scaleIfNeeded() {
    // 后端 CaptureWindowThumbnail 已输出固定 320×180 的 letterbox PNG（原图
    // 等比缩放居中、四周补深色边带），本身不变形、不被裁剪。这里只按卡片
    // 实际显示尺寸等比缩放（KeepAspectRatio：装进框内、不裁剪不变形），
    // 避免之前 KeepAspectRatioByExpanding 把边带内容裁掉。
    if (m_requestedSize.width() > 0 && m_requestedSize.height() > 0 &&
        m_image.size() != m_requestedSize) {
      m_image = m_image.scaled(m_requestedSize, Qt::KeepAspectRatio,
                               Qt::SmoothTransformation);
    }
  }

  qint64 m_hwnd = 0;
  QString m_appName;
  QSize m_requestedSize;
  QImage m_image;
  std::shared_ptr<QGrpcHttp2Channel> m_channel;
  std::unique_ptr<shadowworker::WhitelistService::Client> m_client;
  std::unique_ptr<QGrpcCallReply> m_reply;
};

}  // namespace

QQuickImageResponse *WindowThumbnailProvider::requestImageResponse(
    const QString &id, const QSize &requestedSize) {
  // 立即返回 response；gRPC 异步完成后它自己 emit finished()。
  // 调用线程（image loader）不阻塞，渲染管线不受影响。
  return new ThumbnailResponse(id, requestedSize);
}
