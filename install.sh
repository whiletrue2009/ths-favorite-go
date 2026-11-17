#!/usr/bin/env bash
# ths 一键安装（macOS / Linux）：下载最新版二进制到 ~/.local/bin
set -euo pipefail

REPO="whiletrue2009/ths-favorite-go"
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "不支持的 CPU 架构: $ARCH"; exit 1 ;;
esac

echo ">> 查询最新版本…"
URL="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep -o "https://[^\"]*/ths_[^\"]*_${OS}_${ARCH}\.zip" | head -1)"
if [ -z "$URL" ]; then echo "未找到 ${OS}/${ARCH} 的发布包"; exit 1; fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
echo ">> 下载 $URL"
if ! curl -fsSL --max-time 60 -o "$TMP/ths.zip" "$URL"; then
  echo ">> 直连失败，尝试镜像…"
  curl -fsSL --max-time 120 -o "$TMP/ths.zip" "https://ghproxy.cn/$URL"
fi
unzip -oq "$TMP/ths.zip" -d "$TMP"

BIN_DIR="${HOME}/.local/bin"
mkdir -p "$BIN_DIR"
mv "$TMP/ths" "$BIN_DIR/ths"
chmod +x "$BIN_DIR/ths"

# macOS 解除隔离
if [ "$OS" = "darwin" ]; then
  xattr -d com.apple.quarantine "$BIN_DIR/ths" 2>/dev/null || true
fi

echo ">> 安装完成: $BIN_DIR/ths"
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo "!! 请把 $BIN_DIR 加入 PATH（例如 echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.zshrc && source ~/.zshrc）" ;;
esac
echo ">> 下一步：运行 ths login 登录，然后对 AI 说「帮我看看自选股」"
