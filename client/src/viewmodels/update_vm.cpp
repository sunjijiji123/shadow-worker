// UpdateViewModel 实现：检查更新（gRPC）+ 安装包下载（QNetworkAccessManager）。

#include "update_vm.h"

#include <QDateTime>
#include <QDir>
#include <QFile>
#include <QGrpcCallReply>
#include <QGrpcStatus>
#include <QNetworkAccessManager>
#include <QNetworkReply>
#include <QNetworkRequest>
#include <QStandardPaths>
#include <QUrl>
#include <QProcess>
#include <QCoreApplication>
#include <QFileInfo>

using shadowworker::CheckUpdateRequest;
using shadowworker::CheckUpdateResponse;

UpdateViewModel::UpdateViewModel(QObject *parent) : QObject(parent) {
  m_dailyTimer = new QTimer(this);
  m_dailyTimer->setSingleShot(true);
  connect(m_dailyTimer, &QTimer::timeout, this, &UpdateViewModel::checkUpdate);

  m_nam = new QNetworkAccessManager(this);
}

UpdateViewModel::~UpdateViewModel() {
  if (m_downloadReply) {
    m_downloadReply->abort();
    m_downloadReply->deleteLater();
  }
  if (m_downloadFile) {
    delete m_downloadFile;
  }
}

void UpdateViewModel::setChannel(std::shared_ptr<QAbstractGrpcChannel> channel) {
  m_channel = std::move(channel);
  m_client.attachChannel(m_channel);
}

void UpdateViewModel::setCurrentVersion(const QString &v) {
  if (m_currentVersion == v)
    return;
  m_currentVersion = v;
  emit currentVersionChanged();
}

void UpdateViewModel::setCheckDaily(bool v) {
  if (m_checkDaily == v)
    return;
  m_checkDaily = v;
  if (!m_checkDaily && m_dailyTimer)
    m_dailyTimer->stop();
  emit checkDailyChanged();
}

void UpdateViewModel::checkUpdate() {
  if (!m_channel) {
    setError(QStringLiteral("gRPC channel 未初始化"));
    return;
  }

  setLoading(true);
  setError({});

  CheckUpdateRequest req;
  req.setCurrentVersion(m_currentVersion.isEmpty() ? QStringLiteral("unknown") : m_currentVersion);

  auto reply = m_client.CheckUpdate(req);
  auto *replyPtr = reply.get();
  reply.release();

  QObject::connect(replyPtr, &QGrpcCallReply::finished, this,
                   [this, replyPtr](const QGrpcStatus &status) {
                     replyPtr->deleteLater();
                     setLoading(false);

                     if (!status.isOk()) {
                       setAvailable(false);
                       setError(QStringLiteral("检查更新失败: ") + status.message());
                       return;
                     }

                     auto opt = replyPtr->read<CheckUpdateResponse>();
                     if (!opt.has_value()) {
                       setAvailable(false);
                       setError(QStringLiteral("解析更新响应失败"));
                       return;
                     }

                     const auto &resp = *opt;
                     if (!resp.error().isEmpty()) {
                       setAvailable(false);
                       setError(resp.error());
                     } else if (resp.available()) {
                       setLatestVersion(resp.latestVersion());
                       setDownloadUrl(resp.downloadUrl());
                       setChangelogUrl(resp.changelogUrl());
                       setChangelog(resp.changelog());
                       setPackageSize(resp.packageSize());
                       setPackageSizeText(formatPackageSize(resp.packageSize()));
                       setPublishedAt(resp.publishedAt());
                       setAvailable(true);
                       setError({});
                     } else {
                       setAvailable(false);
                       setLatestVersion({});
                       setDownloadUrl({});
                       setChangelogUrl({});
                       setChangelog({});
                       setPackageSize(0);
                       setPackageSizeText({});
                       setPublishedAt({});
                       setError({});
                     }

                     setLastCheckedAt(QDateTime::currentDateTime().toString(QStringLiteral("yyyy-MM-dd hh:mm")));

                     if (m_checkDaily && m_dailyTimer)
                       m_dailyTimer->start(24 * 60 * 60 * 1000);
                   });
}

void UpdateViewModel::startDownload() {
  if (m_downloadState == QStringLiteral("downloading")) {
    return;  // 防重入
  }
  if (m_downloadUrl.isEmpty()) {
    setError(QStringLiteral("没有可下载的更新"));
    return;
  }

  // 准备下载目录
  QDir().mkpath(downloadsDir());

  // 文件名：从 downloadUrl 提取，兜底用 latestVersion
  QString filename = QFileInfo(QUrl(m_downloadUrl).fileName()).fileName();
  if (filename.isEmpty()) {
    filename = QStringLiteral("ShadowWorker-%1-setup.exe").arg(m_latestVersion);
  }
  QString path = QDir(downloadsDir()).absoluteFilePath(filename);

  // 删除可能存在的旧文件
  QFile::remove(path);

  auto *file = new QFile(path, this);
  if (!file->open(QIODevice::WriteOnly)) {
    setError(QStringLiteral("无法创建下载文件: ") + path);
    delete file;
    setDownloadState(QStringLiteral("failed"));
    return;
  }

  // 关闭上一个未清理的下载（如有）
  if (m_downloadReply) {
    m_downloadReply->abort();
    m_downloadReply->deleteLater();
    m_downloadReply = nullptr;
  }
  if (m_downloadFile) {
    delete m_downloadFile;
  }
  m_downloadFile = file;
  setDownloadedPath(path);
  setDownloadProgress(0);
  setDownloadState(QStringLiteral("downloading"));
  setError({});

  QNetworkRequest req((QUrl(m_downloadUrl)));
  req.setAttribute(QNetworkRequest::RedirectPolicyAttribute, QNetworkRequest::NoLessSafeRedirectPolicy);
  req.setHeader(QNetworkRequest::UserAgentHeader, QStringLiteral("ShadowWorker-Updater/1.0"));

  m_downloadReply = m_nam->get(req);

  // 进度
  connect(m_downloadReply, &QNetworkReply::downloadProgress, this,
          [this](qint64 received, qint64 total) {
            if (total > 0) {
              setDownloadProgress(static_cast<int>(received * 100 / total));
            } else if (m_packageSize > 0) {
              // fallback：服务器预先告知的 packageSize
              setDownloadProgress(static_cast<int>(received * 100 / m_packageSize));
            }
          });

  // 数据落盘
  connect(m_downloadReply, &QIODevice::readyRead, this, [this]() {
    if (m_downloadReply && m_downloadFile) {
      m_downloadFile->write(m_downloadReply->readAll());
    }
  });

  // 完成（成功或失败）
  connect(m_downloadReply, &QNetworkReply::finished, this, [this, path]() {
    if (!m_downloadReply) return;
    QNetworkReply::NetworkError err = m_downloadReply->error();
    m_downloadReply->deleteLater();
    m_downloadReply = nullptr;

    if (m_downloadFile) {
      m_downloadFile->close();
    }

    if (err != QNetworkReply::NoError && err != QNetworkReply::OperationCanceledError) {
      setError(QStringLiteral("下载失败: ") + QVariant::fromValue(err).toString());
      setDownloadState(QStringLiteral("failed"));
      if (m_downloadFile) {
        delete m_downloadFile;
        m_downloadFile = nullptr;
      }
      QFile::remove(path);
      return;
    }

    if (err == QNetworkReply::OperationCanceledError) {
      // 用户取消，状态由 cancelDownload 设置，这里只清理
      if (m_downloadFile) {
        delete m_downloadFile;
        m_downloadFile = nullptr;
      }
      QFile::remove(path);
      return;
    }

    // 校验大小（sha256 目前 GitHub asset 不暴露，只校验大小）
    if (m_packageSize > 0) {
      qint64 actual = QFileInfo(path).size();
      if (actual != m_packageSize) {
        setError(QStringLiteral("下载文件大小不匹配: 期望 %1, 实际 %2")
                     .arg(m_packageSize).arg(actual));
        setDownloadState(QStringLiteral("failed"));
        if (m_downloadFile) {
          delete m_downloadFile;
          m_downloadFile = nullptr;
        }
        QFile::remove(path);
        return;
      }
    }

    if (m_downloadFile) {
      delete m_downloadFile;
      m_downloadFile = nullptr;
    }
    setDownloadProgress(100);
    setDownloadState(QStringLiteral("ready"));
  });
}

void UpdateViewModel::cancelDownload() {
  if (m_downloadReply) {
    m_downloadReply->abort();  // 触发 finished 回调里的 OperationCanceledError 分支
  }
  setDownloadState(QStringLiteral("idle"));
  setDownloadProgress(0);
  setDownloadedPath({});
}

void UpdateViewModel::launchInstaller() {
  if (m_downloadedPath.isEmpty() || !QFileInfo::exists(m_downloadedPath)) {
    setError(QStringLiteral("安装包不存在，请重新下载"));
    return;
  }
  // 拉起安装包（Inno Setup 的 PrivilegesRequired=admin 会触发 UAC 提权；
  // PrepareToInstall 会 taskkill 客户端 + 后端并覆盖文件）。
  // 用 startDetached：脱离父进程生命周期，客户端退出后安装继续。
  bool ok = QProcess::startDetached(m_downloadedPath, QStringList{});
  if (!ok) {
    setError(QStringLiteral("无法启动安装程序: ") + m_downloadedPath);
    return;
  }
  // 通知主程序尽快退出，让 Inno 能覆盖客户端 exe（否则文件被锁）。
  emit requestQuit();
}

QString UpdateViewModel::downloadsDir() const {
  QString base = QStandardPaths::writableLocation(QStandardPaths::AppDataLocation);
  if (base.isEmpty()) {
    base = QStandardPaths::writableLocation(QStandardPaths::GenericDataLocation);
  }
  return QDir(base).absoluteFilePath(QStringLiteral("downloads"));
}

QString UpdateViewModel::formatPackageSize(qint64 bytes) {
  if (bytes < 0)
    return QStringLiteral("0 B");
  if (bytes < 1024)
    return QStringLiteral("%1 B").arg(bytes);
  if (bytes < 1024 * 1024)
    return QStringLiteral("%1 KB").arg(bytes / 1024.0, 0, 'f', 1);
  return QStringLiteral("%1 MB").arg(bytes / (1024.0 * 1024.0), 0, 'f', 1);
}

void UpdateViewModel::setLoading(bool v) {
  if (m_loading == v)
    return;
  m_loading = v;
  emit loadingChanged();
}

void UpdateViewModel::setError(const QString &e) {
  if (m_error == e)
    return;
  m_error = e;
  emit errorChanged();
}

void UpdateViewModel::setAvailable(bool v) {
  if (m_available == v)
    return;
  m_available = v;
  emit availableChanged();
}

void UpdateViewModel::setLatestVersion(const QString &v) {
  if (m_latestVersion == v)
    return;
  m_latestVersion = v;
  emit latestVersionChanged();
}

void UpdateViewModel::setDownloadUrl(const QString &v) {
  if (m_downloadUrl == v)
    return;
  m_downloadUrl = v;
  emit downloadUrlChanged();
}

void UpdateViewModel::setChangelogUrl(const QString &v) {
  if (m_changelogUrl == v)
    return;
  m_changelogUrl = v;
  emit changelogUrlChanged();
}

void UpdateViewModel::setChangelog(const QString &v) {
  if (m_changelog == v)
    return;
  m_changelog = v;
  emit changelogChanged();
}

void UpdateViewModel::setPackageSizeText(const QString &v) {
  if (m_packageSizeText == v)
    return;
  m_packageSizeText = v;
  emit packageSizeTextChanged();
}

void UpdateViewModel::setPackageSize(qint64 v) {
  if (m_packageSize == v)
    return;
  m_packageSize = v;
  emit packageSizeChanged();
}

void UpdateViewModel::setPublishedAt(const QString &v) {
  if (m_publishedAt == v)
    return;
  m_publishedAt = v;
  emit publishedAtChanged();
}

void UpdateViewModel::setLastCheckedAt(const QString &v) {
  if (m_lastCheckedAt == v)
    return;
  m_lastCheckedAt = v;
  emit lastCheckedAtChanged();
}

void UpdateViewModel::setDownloadState(const QString &s) {
  if (m_downloadState == s)
    return;
  m_downloadState = s;
  emit downloadStateChanged();
}

void UpdateViewModel::setDownloadProgress(int p) {
  if (m_downloadProgress == p)
    return;
  m_downloadProgress = p;
  emit downloadProgressChanged();
}

void UpdateViewModel::setDownloadedPath(const QString &p) {
  if (m_downloadedPath == p)
    return;
  m_downloadedPath = p;
  emit downloadedPathChanged();
}
