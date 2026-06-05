#!/usr/bin/env bash
set -euo pipefail

REPO="${AGENT_PACKS_REPO:-sandeshh/agent-packs}"
VERSION="${AGENT_PACKS_VERSION:-latest}"
INSTALL_DIR="${AGENT_PACKS_INSTALL_DIR:-${HOME}/.local/bin}"
INSTALL_SKILL="${AGENT_PACKS_INSTALL_SKILL:-1}"
SKILL_DIR="${AGENT_PACKS_SKILL_DIR:-${HOME}/.codex/skills/agent-packs}"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "${arch}" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) echo "unsupported architecture: ${arch}" >&2; exit 1 ;;
esac

case "${os}" in
  darwin|linux) ;;
  *) echo "unsupported OS: ${os}" >&2; exit 1 ;;
esac

if [ "${VERSION}" = "latest" ]; then
  api="https://api.github.com/repos/${REPO}/releases/latest"
  VERSION="$(curl -fsSL "${api}" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
fi

if [ -z "${VERSION}" ]; then
  echo "could not resolve latest release version" >&2
  exit 1
fi

asset="agent-packs_${os}_${arch}.tar.gz"
url="https://github.com/${REPO}/releases/download/${VERSION}/${asset}"
tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

echo "Installing agent-packs ${VERSION} for ${os}/${arch}"
curl -fsSL "${url}" -o "${tmpdir}/${asset}"
tar -xzf "${tmpdir}/${asset}" -C "${tmpdir}"
mkdir -p "${INSTALL_DIR}"
install -m 755 "${tmpdir}/agent-packs" "${INSTALL_DIR}/agent-packs"

if [ "${INSTALL_SKILL}" != "0" ] && [ -d "${tmpdir}/skills/agent-packs" ]; then
  mkdir -p "${SKILL_DIR}"
  cp -R "${tmpdir}/skills/agent-packs/." "${SKILL_DIR}/"
  echo "Installed Agent Packs skill to ${SKILL_DIR}"
fi

if ! echo ":${PATH}:" | grep -q ":${INSTALL_DIR}:"; then
  echo "Add ${INSTALL_DIR} to your PATH to use agent-packs"
fi

echo "Installed agent-packs to ${INSTALL_DIR}/agent-packs"
"${INSTALL_DIR}/agent-packs" version
