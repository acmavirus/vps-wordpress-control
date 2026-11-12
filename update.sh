#!/usr/bin/env bash
# ==============================================================================
# VPS WordPress Control - Kịch bản cập nhật nhanh 1 dòng lệnh
# ==============================================================================

set -e

GITHUB_REPO="${GITHUB_REPO:-acmavirus/vps-wordpress-control}"
INSTALL_DIR="/opt/wp-control"
SERVICE_NAME="wp-control"

# Màu sắc hiển thị
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}>>> Đang cập nhật VPS WordPress Control lên bản mới nhất...${NC}"

# Kiểm tra quyền root
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}[Lỗi] Kịch bản yêu cầu quyền root. Vui lòng chạy: sudo bash $0${NC}"
    exit 1
fi

# Nhận diện kiến trúc
ARCH=$(uname -m)
case $ARCH in
    x86_64) BIN_ARCH="amd64" ;;
    aarch64|arm64) BIN_ARCH="arm64" ;;
    *) echo -e "${RED}[Lỗi] Kiến trúc $ARCH chưa được hỗ trợ.${NC}"; exit 1 ;;
esac

TMP_FILE=$(mktemp /tmp/wp-control-update.XXXXXX.tar.gz)
# Tải từ tag cụ thể nếu có truyền đối số hoặc tự động tìm tag mới nhất từ API để tránh CDN cache của releases/latest
TARGET_TAG="${1:-}"
if [ -z "$TARGET_TAG" ]; then
    TARGET_TAG=$(curl -sSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" 2>/dev/null | grep '"tag_name":' | head -n 1 | cut -d '"' -f 4)
fi

if [ -n "$TARGET_TAG" ]; then
    DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/${TARGET_TAG}/wp-control-linux-${BIN_ARCH}.tar.gz"
    echo -e "${GREEN}[1/3] Tải phiên bản ${TARGET_TAG} (${BIN_ARCH})...${NC}"
else
    DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/latest/download/wp-control-linux-${BIN_ARCH}.tar.gz"
    echo -e "${GREEN}[1/3] Tải bản phát hành mới nhất (${BIN_ARCH})...${NC}"
fi

if ! curl -sSLf "${DOWNLOAD_URL}" -o "${TMP_FILE}"; then
    echo -e "${YELLOW}[Cảnh báo] Thử fallback về releases/latest...${NC}"
    FALLBACK_URL="https://github.com/${GITHUB_REPO}/releases/latest/download/wp-control-linux-${BIN_ARCH}.tar.gz"
    if ! curl -sSLf "${FALLBACK_URL}" -o "${TMP_FILE}"; then
        echo -e "${RED}[Lỗi] Không thể tải bản cập nhật từ GitHub. Vui lòng thử lại sau.${NC}"
        rm -f "${TMP_FILE}"
        exit 1
    fi
fi

echo -e "${GREEN}[2/3] Dừng dịch vụ và thay thế binary...${NC}"
systemctl stop "${SERVICE_NAME}" 2>/dev/null || true

mkdir -p "${INSTALL_DIR}/bin" "${INSTALL_DIR}/templates"
tar -xzf "${TMP_FILE}" -C "${INSTALL_DIR}/bin/"
if [ -d "${INSTALL_DIR}/bin/templates" ]; then
    cp -rn "${INSTALL_DIR}/bin/templates/"* "${INSTALL_DIR}/templates/" 2>/dev/null || true
    rm -rf "${INSTALL_DIR}/bin/templates"
fi
# Đảm bảo template Nginx tồn tại
if [ ! -f "${INSTALL_DIR}/templates/nginx-wp.conf" ]; then
    curl -sSLf "https://raw.githubusercontent.com/${GITHUB_REPO}/main/templates/nginx-wp.conf" -o "${INSTALL_DIR}/templates/nginx-wp.conf" 2>/dev/null || true
fi
chmod +x "${INSTALL_DIR}/bin/wp-control"
rm -f "${TMP_FILE}"

echo -e "${GREEN}[3/3] Khởi động lại dịch vụ và kiểm tra Firewall...${NC}"
systemctl start "${SERVICE_NAME}"

# Đảm bảo cổng trong .env được mở trên UFW nếu UFW đang bật
ENV_FILE="${INSTALL_DIR}/.env"
if [ -f "$ENV_FILE" ]; then
    ACTIVE_PORT=$(grep -E "^APP_PORT=" "$ENV_FILE" 2>/dev/null | cut -d '=' -f2 | tr -d '"' | tr -d ' ' || true)
    if [ -n "$ACTIVE_PORT" ] && command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "Status: active"; then
        ufw allow ${ACTIVE_PORT}/tcp >/dev/null 2>&1
    fi
fi

if systemctl is-active --quiet "${SERVICE_NAME}"; then
    echo -e "${GREEN}✓ CẬP NHẬT THÀNH CÔNG! Dịch vụ đang hoạt động ổn định.${NC}"
else
    echo -e "${YELLOW}Dịch vụ chưa chạy, thử restart...${NC}"
    systemctl restart "${SERVICE_NAME}"
fi

