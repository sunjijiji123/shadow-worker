#!/usr/bin/env bash
# 在 Linux 服务器上安装/更新 Shadow Worker update_service
# 用法（root）：bash scripts/install.sh
set -euo pipefail

INSTALL_DIR="/opt/shadow-worker-update"
CONFIG_DIR="/etc/shadow-worker-update"
DATA_DIR="/var/lib/shadow-worker-update/data"
SERVICE_NAME="shadow-worker-update"

if [ "$EUID" -ne 0 ]; then
    echo "请使用 root 权限运行：sudo bash scripts/install.sh"
    exit 1
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIST="$ROOT/dist/linux-amd64"

if [ ! -f "$DIST/server" ]; then
    echo "未找到编译产物，先执行：bash scripts/build-linux.sh"
    exit 1
fi

# 停掉旧服务
if systemctl list-unit-files | grep -q "^$SERVICE_NAME"; then
    systemctl stop "$SERVICE_NAME" || true
fi

# 安装二进制和静态资源
mkdir -p "$INSTALL_DIR"
cp "$DIST/server" "$INSTALL_DIR/server"
rm -rf "$INSTALL_DIR/web"
cp -r "$DIST/web" "$INSTALL_DIR/web"
chmod +x "$INSTALL_DIR/server"

# 配置目录
mkdir -p "$CONFIG_DIR"
mkdir -p "$DATA_DIR"

# 首次安装时写入默认配置
if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
    cat > "$CONFIG_DIR/config.yaml" <<EOF
listen_addr: "0.0.0.0:8080"
data_dir: "$DATA_DIR"

admin_username: "admin"
admin_password: "changeme"

jwt_secret: "$(openssl rand -hex 32)"

github_owner: "sunjijiji123"
github_repo: "shadow-worker"
github_cache_ttl: "5m"
asset_name_template: "ShadowWorker-{version}-setup.exe"
EOF
    chmod 600 "$CONFIG_DIR/config.yaml"
    echo "已生成配置文件：$CONFIG_DIR/config.yaml，请修改 admin_password / jwt_secret / github_owner 等"
fi

# 环境变量文件（用于 UPDATE_GITHUB_TOKEN 等敏感信息）
if [ ! -f "$CONFIG_DIR/env" ]; then
    cat > "$CONFIG_DIR/env" <<EOF
# UPDATE_GITHUB_TOKEN=ghp_xxxxxxxx
EOF
    chmod 600 "$CONFIG_DIR/env"
    echo "已生成环境变量文件：$CONFIG_DIR/env，如需 token 请取消注释并填入"
fi

# systemd unit
cat > "/etc/systemd/system/$SERVICE_NAME.service" <<EOF
[Unit]
Description=Shadow Worker Update Service
After=network.target

[Service]
Type=simple
ExecStart=$INSTALL_DIR/server
WorkingDirectory=$INSTALL_DIR
Environment="UPDATE_CONFIG=$CONFIG_DIR/config.yaml"
EnvironmentFile=-$CONFIG_DIR/env
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable "$SERVICE_NAME"
systemctl start "$SERVICE_NAME"

sleep 1
if systemctl is-active --quiet "$SERVICE_NAME"; then
    echo "$SERVICE_NAME 安装完成，运行中："
    systemctl status "$SERVICE_NAME" --no-pager
else
    echo "服务启动失败，请查看日志：journalctl -u $SERVICE_NAME -n 50"
    exit 1
fi
