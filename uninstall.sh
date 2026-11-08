#!/usr/bin/env bash
# ==============================================================================
# VPS WordPress Control - Kịch bản gỡ cài đặt sạch sẽ hoàn toàn
# ==============================================================================

set -e

INSTALL_DIR="/opt/wp-control"
SERVICE_NAME="wp-control"

# Màu sắc hiển thị
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${RED}======================================================${NC}"
echo -e "${RED}      GỠ CÀI ĐẶT VPS WORDPRESS CONTROL PANEL         ${NC}"
echo -e "${RED}======================================================${NC}"

# Kiểm tra quyền root
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}[Lỗi] Kịch bản yêu cầu quyền root. Vui lòng chạy: sudo bash $0${NC}"
    exit 1
fi

# 1. Đọc cổng đang dùng từ .env để dọn dẹp tường lửa nếu có
if [ -f "${INSTALL_DIR}/.env" ]; then
    ACTIVE_PORT=$(grep -E "^APP_PORT=" "${INSTALL_DIR}/.env" 2>/dev/null | cut -d '=' -f2 | tr -d '"' | tr -d ' ' || true)
fi

# 2. Dừng và vô hiệu hóa service
echo -e "${YELLOW}[1/4] Dừng và vô hiệu hóa Systemd service (${SERVICE_NAME})...${NC}"
systemctl stop ${SERVICE_NAME} 2>/dev/null || true
systemctl disable ${SERVICE_NAME} 2>/dev/null || true
rm -f /etc/systemd/system/${SERVICE_NAME}.service
systemctl daemon-reload
systemctl reset-failed

# 3. Xóa thư mục cài đặt và dữ liệu
echo -e "${YELLOW}[2/4] Xóa thư mục cài đặt ${INSTALL_DIR}...${NC}"
rm -rf ${INSTALL_DIR}

# 4. Dọn dẹp tường lửa Firewall nếu có mở cổng
echo -e "${YELLOW}[3/4] Dọn dẹp cấu hình Firewall...${NC}"
if [ -n "$ACTIVE_PORT" ]; then
    if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "Status: active"; then
        ufw delete allow ${ACTIVE_PORT}/tcp >/dev/null 2>&1 || true
        echo -e "${GREEN}  ✓ Đã đóng cổng ${ACTIVE_PORT}/tcp trên UFW.${NC}"
    fi
    if command -v firewall-cmd >/dev/null 2>&1 && systemctl is-active --quiet firewalld 2>/dev/null; then
        firewall-cmd --permanent --remove-port=${ACTIVE_PORT}/tcp >/dev/null 2>&1 || true
        firewall-cmd --reload >/dev/null 2>&1 || true
        echo -e "${GREEN}  ✓ Đã đóng cổng ${ACTIVE_PORT}/tcp trên Firewalld.${NC}"
    fi
fi

echo -e "${GREEN}[4/4] Hoàn tất dọn dẹp hệ thống!${NC}"
echo -e ""
echo -e "${GREEN}======================================================${NC}"
echo -e "${GREEN}  ĐÃ GỠ CÀI ĐẶT SẠCH SẼ HOÀN TOÀN VPS WORDPRESS CONTROL  ${NC}"
echo -e "${GREEN}======================================================${NC}"

