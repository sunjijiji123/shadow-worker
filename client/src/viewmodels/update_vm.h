// UpdateViewModel: 软件更新检查 + 安装包下载 ↔ UpdateService gRPC 桥
//
// 下载流程：发现更新 → startDownload() 用 QNetworkAccessManager 从 GitHub
// 直链下载到 %APPDATA%/shadow-worker/downloads/ → 进度驱动圆环 UI →
// 下载完成校验大小 → launchInstaller() 拉起 Inno Setup 安装包并自退出。
// Inno 的 PrepareToInstall 会杀掉残留进程并覆盖文件（见 ShadowWorker.iss）。

#pragma once

#include <QObject>
#include <QString>
#include <QTimer>
#include <memory>

#include "update.qpb.h"
#include "update_client.grpc.qpb.h"
#include <QAbstractGrpcChannel>

class QNetworkAccessManager;
class QNetworkReply;
class QIODevice;

class UpdateViewModel : public QObject {
  Q_OBJECT

  // ---- 检查更新相关 ----
  Q_PROPERTY(bool loading READ loading NOTIFY loadingChanged)
  Q_PROPERTY(QString error READ error NOTIFY errorChanged)
  Q_PROPERTY(bool available READ available NOTIFY availableChanged)
  Q_PROPERTY(QString latestVersion READ latestVersion NOTIFY latestVersionChanged)
  Q_PROPERTY(QString downloadUrl READ downloadUrl NOTIFY downloadUrlChanged)
  Q_PROPERTY(QString changelogUrl READ changelogUrl NOTIFY changelogUrlChanged)
  Q_PROPERTY(QString changelog READ changelog NOTIFY changelogChanged)
  Q_PROPERTY(QString packageSizeText READ packageSizeText NOTIFY packageSizeTextChanged)
  Q_PROPERTY(qint64 packageSize READ packageSize NOTIFY packageSizeChanged)
  Q_PROPERTY(QString publishedAt READ publishedAt NOTIFY publishedAtChanged)
  Q_PROPERTY(QString lastCheckedAt READ lastCheckedAt NOTIFY lastCheckedAtChanged)
  Q_PROPERTY(QString currentVersion READ currentVersion WRITE setCurrentVersion NOTIFY currentVersionChanged)
  Q_PROPERTY(bool checkDaily READ checkDaily WRITE setCheckDaily NOTIFY checkDailyChanged)

  // ---- 下载安装相关 ----
  // downloadState: idle | downloading | ready | failed
  Q_PROPERTY(QString downloadState READ downloadState NOTIFY downloadStateChanged)
  Q_PROPERTY(int downloadProgress READ downloadProgress NOTIFY downloadProgressChanged)
  Q_PROPERTY(QString downloadedPath READ downloadedPath NOTIFY downloadedPathChanged)

public:
  explicit UpdateViewModel(QObject *parent = nullptr);
  ~UpdateViewModel() override;

  void setChannel(std::shared_ptr<QAbstractGrpcChannel> channel);

  bool loading() const { return m_loading; }
  QString error() const { return m_error; }
  bool available() const { return m_available; }
  QString latestVersion() const { return m_latestVersion; }
  QString downloadUrl() const { return m_downloadUrl; }
  QString changelogUrl() const { return m_changelogUrl; }
  QString changelog() const { return m_changelog; }
  QString packageSizeText() const { return m_packageSizeText; }
  qint64 packageSize() const { return m_packageSize; }
  QString publishedAt() const { return m_publishedAt; }
  QString lastCheckedAt() const { return m_lastCheckedAt; }
  QString currentVersion() const { return m_currentVersion; }
  bool checkDaily() const { return m_checkDaily; }

  QString downloadState() const { return m_downloadState; }
  int downloadProgress() const { return m_downloadProgress; }
  QString downloadedPath() const { return m_downloadedPath; }

  void setCurrentVersion(const QString &v);
  void setCheckDaily(bool v);

  // 检查更新（gRPC）
  Q_INVOKABLE void checkUpdate();

  // 开始下载安装包（GitHub 直链）。下载前要求 available=true 且 downloadUrl 非空。
  Q_INVOKABLE void startDownload();
  // 拉起已下载的安装包并退出客户端，让 Inno Setup 接管覆盖安装。
  Q_INVOKABLE void launchInstaller();
  // 取消正在进行的下载。
  Q_INVOKABLE void cancelDownload();

signals:
  void loadingChanged();
  void errorChanged();
  void availableChanged();
  void latestVersionChanged();
  void downloadUrlChanged();
  void changelogUrlChanged();
  void changelogChanged();
  void packageSizeTextChanged();
  void packageSizeChanged();
  void publishedAtChanged();
  void lastCheckedAtChanged();
  void currentVersionChanged();
  void checkDailyChanged();
  void downloadStateChanged();
  void downloadProgressChanged();
  void downloadedPathChanged();
  // 请求退出客户端（launchInstaller 拉起 setup 后发出，main.cpp 监听后 quit）
  void requestQuit();

private:
  void setLoading(bool v);
  void setError(const QString &e);
  void setAvailable(bool v);
  void setLatestVersion(const QString &v);
  void setDownloadUrl(const QString &v);
  void setChangelogUrl(const QString &v);
  void setChangelog(const QString &v);
  void setPackageSizeText(const QString &v);
  void setPackageSize(qint64 v);
  void setPublishedAt(const QString &v);
  void setLastCheckedAt(const QString &v);
  void setDownloadState(const QString &s);
  void setDownloadProgress(int p);
  void setDownloadedPath(const QString &p);

  static QString formatPackageSize(qint64 bytes);
  QString downloadsDir() const;

  shadowworker::UpdateService::Client m_client;
  std::shared_ptr<QAbstractGrpcChannel> m_channel;
  QTimer *m_dailyTimer = nullptr;

  // 网络下载
  QNetworkAccessManager *m_nam = nullptr;
  QNetworkReply *m_downloadReply = nullptr;
  QIODevice *m_downloadFile = nullptr;

  bool m_loading = false;
  QString m_error;
  bool m_available = false;
  QString m_latestVersion;
  QString m_downloadUrl;
  QString m_changelogUrl;
  QString m_changelog;
  QString m_packageSizeText;
  qint64 m_packageSize = 0;
  QString m_publishedAt;
  QString m_lastCheckedAt;
  QString m_currentVersion;
  bool m_checkDaily = false;

  QString m_downloadState = QStringLiteral("idle");
  int m_downloadProgress = 0;
  QString m_downloadedPath;
};
