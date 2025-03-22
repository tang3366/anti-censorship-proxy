#!/bin/bash

# 脚本名称: install.sh
# 功能: 自动安装 Go 环境，下载并运行 anti-censorship-proxy

# 检查是否以 root 权限运行
if [ "$EUID" -ne 0 ]; then
  echo "请以 root 权限运行此脚本 (sudo ./install.sh)"
  exit 1
fi

# 定义变量
GO_VERSION="1.21.8"
REPO_URL="https://github.com/<your-username>/anti-censorship-proxy.git"  # 替换为你的仓库地址
INSTALL_DIR="/usr/local/anti-censorship-proxy"
SERVER_PORT="8080"
KEY="my-secret-key-123"  # 默认密钥，可通过参数修改

# 更新系统并安装依赖
echo "更新系统包..."
apt update -y && apt install -y git curl

# 安装 Go
echo "安装 Go $GO_VERSION..."
curl -LO "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz"
tar -C /usr/local -xzf "go${GO_VERSION}.linux-amd64.tar.gz"
rm "go${GO_VERSION}.linux-amd64.tar.gz"

# 设置 Go 环境变量
if ! grep -q "/usr/local/go/bin" /etc/profile; then
  echo "export PATH=\$PATH:/usr/local/go/bin" >> /etc/profile
fi
source /etc/profile

# 验证 Go 安装
if ! command -v go &> /dev/null; then
  echo "Go 安装失败"
  exit 1
fi
echo "Go 版本: $(go version)"

# 创建安装目录并下载代码
echo "下载代码到 $INSTALL_DIR..."
mkdir -p "$INSTALL_DIR"
git clone "$REPO_URL" "$INSTALL_DIR"
cd "$INSTALL_DIR"

# 编译程序
echo "编译 proxy..."
go build -o proxy proxy.go
if [ $? -ne 0 ]; then
  echo "编译失败"
  exit 1
fi

# 配置防火墙（开放端口）
echo "配置防火墙，开放端口 $SERVER_PORT..."
ufw allow "$SERVER_PORT/tcp"
ufw status

# 创建 systemd 服务（仅服务器端）
echo "创建 systemd 服务..."
cat << EOF > /etc/systemd/system/anti-censorship-proxy.service
[Unit]
Description=Anti-Censorship Proxy Service
After=network.target

[Service]
ExecStart=$INSTALL_DIR/proxy server 0.0.0.0:$SERVER_PORT
Restart=always
User=nobody

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable anti-censorship-proxy
systemctl start anti-censorship-proxy

# 检查服务状态
if systemctl is-active anti-censorship-proxy > /dev/null; then
  echo "服务器端已启动，监听 0.0.0.0:$SERVER_PORT"
else
  echo "服务器启动失败，请检查日志: journalctl -u anti-censorship-proxy"
  exit 1
fi

# 提示客户端安装
echo "服务器安装完成！"
echo "客户端安装：在另一台机器上运行以下命令："
echo "  git clone $REPO_URL"
echo "  cd anti-censorship-proxy"
echo "  go build -o proxy proxy.go"
echo "  ./proxy client <服务器IP>:$SERVER_PORT <目标IP> <目标端口> \"测试消息\""
