#!/bin/bash
set -euo pipefail

GO_VERSION="1.26.2"
GO_TARBALL="go${GO_VERSION}.linux-amd64.tar.gz"
GO_URL="https://go.dev/dl/${GO_TARBALL}"

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run with sudo: sudo ./install_go.sh"
  exit 1
fi

install_prereqs() {
  if command -v dnf >/dev/null 2>&1; then
    dnf install -y wget git
  elif command -v yum >/dev/null 2>&1; then
    yum install -y wget git
  elif command -v apt-get >/dev/null 2>&1; then
    apt-get update
    apt-get install -y wget git
  elif command -v zypper >/dev/null 2>&1; then
    zypper --non-interactive install wget git
  elif command -v apk >/dev/null 2>&1; then
    apk add --no-cache wget git
  else
    echo "No supported package manager found (dnf/yum/apt/zypper/apk)."
    exit 1
  fi
}

install_prereqs

wget -O "${GO_TARBALL}" "${GO_URL}"
rm -rf /usr/local/go
tar -C /usr/local -xzf "${GO_TARBALL}"

ln -sf /usr/local/go/bin/go /usr/local/bin/go
ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt

cat > /etc/profile.d/go.sh <<'EOF'
export PATH=$PATH:/usr/local/go/bin
EOF
chmod 644 /etc/profile.d/go.sh

echo "Installed: wget, git, and Go ${GO_VERSION}"
go version
git --version
wget --version | head -n 1