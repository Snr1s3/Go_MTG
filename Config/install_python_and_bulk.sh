#!/bin/sh
set -eu

BULK_DATA="https://data.scryfall.io/all-cards/all-cards-20260506092337.json"

if [ "$(id -u)" -ne 0 ]; then
  echo "Run with sudo: sudo ./install_python_and_bulk.sh"
  exit 1
fi

install_prereqs() {
  if command -v dnf >/dev/null 2>&1; then
    dnf install -y wget git python3 python3-pip
  elif command -v yum >/dev/null 2>&1; then
    yum install -y wget git python3 python3-pip
  elif command -v apt-get >/dev/null 2>&1; then
    apt-get update
    apt-get install -y wget git python3 python3-pip
  elif command -v zypper >/dev/null 2>&1; then
    zypper --non-interactive install wget git python3 python3-pip
  elif command -v apk >/dev/null 2>&1; then
    apk add --no-cache wget git python3 py3-pip
  else
    echo "No supported package manager found (dnf/yum/apt/zypper/apk)."
    exit 1
  fi
}

install_prereqs

# Install Python packages used by the API service.
pip3 install --no-upgrade pip 2>/dev/null || true
pip3 install fastapi uvicorn[standard]

echo "Installed Python dependencies"
python3 --version
pip3 show fastapi | grep Version

echo "Downloading bulk data"
mkdir -p data
cd data
wget "${BULK_DATA}"
