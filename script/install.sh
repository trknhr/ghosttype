#!/usr/bin/env bash
set -e

REPO="trknhr/ghosttype"
APP="ghosttype"
OS=$(uname | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# x86_64 → amd64, aarch64 → arm64
if [ "$ARCH" = "x86_64" ]; then ARCH=amd64; fi
if [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then ARCH=arm64; fi

echo "🔍 Fetching latest release..."
TAG=$(curl -s https://api.github.com/repos/$REPO/releases/latest | grep tag_name | cut -d '"' -f 4)

echo "⬇️  Downloading $APP $TAG for $OS/$ARCH..."
FILENAME="${APP}_${TAG}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/$TAG/$FILENAME"
echo "📦 URL: $URL"
curl -L "$URL" -o "${APP}.tar.gz"

echo "📦 Extracting..."
tar -xzf "${APP}.tar.gz"
rm "${APP}.tar.gz"

echo "🚀 Installing..."
chmod +x $APP
sudo mv $APP /usr/local/bin/

echo "✅ Installed $APP $TAG"
