#!/bin/sh
set -eu

BULK_DATA="https://data.scryfall.io/all-cards/all-cards-20260506092337.json"
SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
REPO_ROOT="$(CDPATH= cd -- "${SCRIPT_DIR}/.." && pwd)"
API_DIR="${REPO_ROOT}/API"
VENV_DIR="${API_DIR}/.venv"
DATA_DIR="${REPO_ROOT}/API/Data"

SUDO=""
if [ "$(id -u)" -ne 0 ]; then
  if command -v sudo >/dev/null 2>&1; then
    SUDO="sudo"
  fi
fi

run_pkg_cmd() {
  if [ -n "${SUDO}" ]; then
    ${SUDO} "$@"
  else
    "$@"
  fi
}

install_prereqs() {
  if command -v apt-get >/dev/null 2>&1; then
    run_pkg_cmd apt-get update
    run_pkg_cmd apt-get install -y wget git python3 python3-pip python3-venv
  else
    echo "No supported package manager found (dnf/yum/apt/zypper/apk)."
    exit 1
  fi
}

install_prereqs

# Install Python packages used by the API service in a project virtualenv.
mkdir -p "${API_DIR}" "${DATA_DIR}"
if [ ! -d "${VENV_DIR}" ]; then
  if ! python3 -m venv "${VENV_DIR}"; then
    echo "Could not create virtual environment at ${VENV_DIR}."
    echo "On Debian/Ubuntu, install python3-venv and rerun."
    exit 1
  fi
fi

"${VENV_DIR}/bin/python" -m pip install --upgrade pip
"${VENV_DIR}/bin/pip" install fastapi "uvicorn[standard]"

echo "Installed Python dependencies"
"${VENV_DIR}/bin/python" --version
"${VENV_DIR}/bin/pip" show fastapi | grep Version || true

echo "Downloading bulk data"
wget -O "${DATA_DIR}/all-cards.gz" "${BULK_DATA}"
zcat "${DATA_DIR}/all-cards.gz" > "${DATA_DIR}/all-cards.jsonl"
echo "Virtual environment: ${VENV_DIR}"
echo "Activate with: . ${VENV_DIR}/bin/activate"
