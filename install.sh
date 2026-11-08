#!/usr/bin/env bash
# ==============================================================================
# VPS WordPress Control - Kịch bản cài đặt tự động 1-Click từ GitHub
# Hỗ trợ: Ubuntu 20.04/22.04/24.04 LTS, Debian 11/12
# ==============================================================================

set -e

# Cấu hình GitHub Repository
GITHUB_REPO="${GITHUB_REPO:-acmavirus/vps-wordpress-control}"
INSTALL_DIR="/opt/wp-control"
SERVICE_NAME="wp-control"
DEFAULT_PORT="9090"

# Màu sắc hiển thị
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}======================================================${NC}"
echo -e "${BLUE}    CÀI ĐẶT VPS WORDPRESS CONTROL (DEDICATED PANEL)   ${NC}"
echo -e "${BLUE}======================================================${NC}"

# 1. Kiểm tra quyền root
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}[Lỗi] Kịch bản yêu cầu quyền root. Vui lòng chạy: sudo bash $0${NC}"
    exit 1
fi

# 2. Nhận diện kiến trúc CPU
ARCH=$(uname -m)
case $ARCH in
    x86_64)
        BIN_ARCH="amd64"
        ;;
    aarch64|arm64)
        BIN_ARCH="arm64"
        ;;
    *)
        echo -e "${RED}[Lỗi] Kiến trúc CPU $ARCH hiện chưa được hỗ trợ.${NC}"
        exit 1
        ;;
esac
echo -e "${GREEN}[1/7] Phát hiện kiến trúc: ${BIN_ARCH}${NC}"

# 3. Cài đặt các gói phụ trợ cần thiết
echo -e "${GREEN}[2/7] Kiểm tra và cập nhật các gói hệ thống...${NC}"
apt-get update -qq
apt-get install -y -qq curl tar openssl ufw > /dev/null 2>&1

# 4. Tạo cấu trúc thư mục cài đặt
echo -e "${GREEN}[3/7] Thiết lập thư mục tại ${INSTALL_DIR}...${NC}"
mkdir -p ${INSTALL_DIR}/bin
mkdir -p ${INSTALL_DIR}/data
mkdir -p ${INSTALL_DIR}/logs
mkdir -p ${INSTALL_DIR}/templates

# 5. Tải file thực thi (Binary Release) từ GitHub
echo -e "${GREEN}[4/7] Đang tải bản phát hành mới nhất từ GitHub (${GITHUB_REPO})...${NC}"
DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/latest/download/wp-control-linux-${BIN_ARCH}.tar.gz"

TMP_DIR=$(mktemp -d)
if curl -sSLf "${DOWNLOAD_URL}" -o "${TMP_DIR}/wp-control.tar.gz"; then
    tar -xzf "${TMP_DIR}/wp-control.tar.gz" -C ${INSTALL_DIR}/bin/
    chmod +x ${INSTALL_DIR}/bin/wp-control
    rm -rf "${TMP_DIR}"
else
    echo -e "${YELLOW}[Cảnh báo] Chưa tìm thấy GitHub Release tại: ${DOWNLOAD_URL}${NC}"
    echo -e "${YELLOW}Nếu bạn đang tự build mã nguồn, vui lòng đặt binary tại: ${INSTALL_DIR}/bin/wp-control${NC}"
fi

# 6. Kiểm tra cổng mạng khả dụng và sinh/cập nhật file cấu hình .env
echo -e "${GREEN}[5/7] Kiểm tra cổng mạng khả dụng và cấu hình .env...${NC}"
ENV_FILE="${INSTALL_DIR}/.env"

# Hàm kiểm tra cổng xem có tiến trình nào đang chiếm dụng không
is_port_in_use() {
    local check_port=$1
    if command -v ss >/dev/null 2>&1; then
        ss -tulpn 2>/dev/null | grep -qE "[:.]${check_port}\b"
    elif command -v netstat >/dev/null 2>&1; then
        netstat -tulpn 2>/dev/null | grep -qE "[:.]${check_port}\b"
    elif command -v lsof >/dev/null 2>&1; then
        lsof -iTCP:${check_port} -sTCP:LISTEN >/dev/null 2>&1
    else
        (timeout 1 bash -c "cat < /dev/null > /dev/tcp/127.0.0.1/${check_port}") 2>/dev/null
    fi
}

# Tạm dừng service cũ nếu đang chạy để không tự xung đột cổng với chính nó
systemctl stop ${SERVICE_NAME} 2>/dev/null || true

TARGET_PORT="${PORT:-$DEFAULT_PORT}"
# Nếu .env đã tồn tại từ trước, đọc cổng đã lưu ra kiểm tra
if [ -f "$ENV_FILE" ]; then
    SAVED_PORT=$(grep -E "^APP_PORT=" "$ENV_FILE" 2>/dev/null | cut -d '=' -f2 | tr -d '"' | tr -d ' ' || true)
    if [ -n "$SAVED_PORT" ]; then
        TARGET_PORT="$SAVED_PORT"
    fi
fi

# Tự động quét tìm cổng khả dụng nếu cổng mục tiêu bị dịch vụ khác chiếm dụng
ORIGINAL_PORT="$TARGET_PORT"
while is_port_in_use "$TARGET_PORT"; do
    echo -e "${YELLOW}  [Cảnh báo] Cổng ${TARGET_PORT} đang bị chiếm dụng bởi tiến trình khác.${NC}"
    TARGET_PORT=$((TARGET_PORT + 1))
    echo -e "${YELLOW}  → Tự động quét kiểm tra cổng tiếp theo: ${TARGET_PORT}...${NC}"
done

if [ "$TARGET_PORT" -ne "$ORIGINAL_PORT" ]; then
    echo -e "${GREEN}  ✓ Đã tự động chọn cổng khả dụng mới: ${TARGET_PORT} (thay cho cổng cũ ${ORIGINAL_PORT})${NC}"
else
    echo -e "${GREEN}  ✓ Cổng ${TARGET_PORT} khả dụng.${NC}"
fi

if [ ! -f "$ENV_FILE" ]; then
    RANDOM_JWT=$(openssl rand -hex 24)
    RANDOM_PASS=$(openssl rand -base64 12 | tr -dc 'a-zA-Z0-9' | head -c 12)

    cat <<EOF > "$ENV_FILE"
APP_NAME="VPS WordPress Control"
APP_ENV=production
APP_PORT=${TARGET_PORT}
APP_HOST=0.0.0.0

JWT_SECRET=${RANDOM_JWT}
ADMIN_DEFAULT_USER=admin
ADMIN_DEFAULT_PASSWORD=${RANDOM_PASS}

DATA_DIR=${INSTALL_DIR}/data
DB_NAME=wpcontrol.db

NGINX_SITES_DIR=/etc/nginx/sites-enabled
NGINX_TEMPLATES_DIR=${INSTALL_DIR}/templates
WWW_ROOT_DIR=/var/www
LOG_DIR=${INSTALL_DIR}/logs
EOF
    chmod 600 "$ENV_FILE"
else
    # Cập nhật cổng mới vào .env nếu có thay đổi
    sed -i "s/^APP_PORT=.*/APP_PORT=${TARGET_PORT}/" "$ENV_FILE"
fi

ACTIVE_PORT="$TARGET_PORT"

# 7. Cấu hình Systemd Service
echo -e "${GREEN}[6/7] Cấu hình Systemd Service (${SERVICE_NAME})...${NC}"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"

cat <<EOF > "$SERVICE_FILE"
[Unit]
Description=VPS WordPress Control Panel Service
After=network.target nginx.service

[Service]
Type=simple
User=root
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/bin/wp-control
Restart=always
RestartSec=5
EnvironmentFile=${INSTALL_DIR}/.env

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload

if [ -f "${INSTALL_DIR}/bin/wp-control" ]; then
    systemctl enable ${SERVICE_NAME} --quiet
    systemctl restart ${SERVICE_NAME}
    sleep 1

    if systemctl is-active --quiet ${SERVICE_NAME}; then
        echo -e "${GREEN}  ✓ Dịch vụ đã được kích hoạt và khởi động thành công!${NC}"
    else
        echo -e "${RED}  ✗ Dịch vụ khởi động thất bại. Log chi tiết:${NC}"
        journalctl -u ${SERVICE_NAME} -n 10 --no-pager
    fi
fi

# 8. Mở cổng Firewall (UFW & Firewalld)
echo -e "${GREEN}[7/7] Cấu hình tường lửa Firewall cho cổng ${ACTIVE_PORT}/tcp...${NC}"
FIREWALL_CONFIGURED=0

if command -v ufw >/dev/null 2>&1; then
    if ufw status 2>/dev/null | grep -q "Status: active"; then
        ufw allow ${ACTIVE_PORT}/tcp >/dev/null 2>&1
        echo -e "${GREEN}  ✓ Đã mở cổng ${ACTIVE_PORT}/tcp trên tường lửa UFW.${NC}"
        FIREWALL_CONFIGURED=1
    else
        echo -e "${BLUE}  ℹ Tường lửa UFW đang ở trạng thái tắt (inactive).${NC}"
    fi
fi

if command -v firewall-cmd >/dev/null 2>&1 && systemctl is-active --quiet firewalld 2>/dev/null; then
    firewall-cmd --permanent --add-port=${ACTIVE_PORT}/tcp >/dev/null 2>&1
    firewall-cmd --reload >/dev/null 2>&1
    echo -e "${GREEN}  ✓ Đã mở cổng ${ACTIVE_PORT}/tcp trên Firewalld.${NC}"
    FIREWALL_CONFIGURED=1
fi

if [ $FIREWALL_CONFIGURED -eq 0 ]; then
    echo -e "${GREEN}  ✓ Tường lửa không chặn cổng hoặc đã được cấu hình mở.${NC}"
fi

# Lấy địa chỉ IP máy chủ
SERVER_IP=$(curl -s -4 icanhazip.com || hostname -I | awk '{print $1}')
ADMIN_PASS=$(grep "ADMIN_DEFAULT_PASSWORD=" "$ENV_FILE" | cut -d '=' -f2)

echo -e ""
echo -e "${GREEN}======================================================${NC}"
echo -e "${GREEN}       CÀI ĐẶT THÀNH CÔNG VPS WORDPRESS CONTROL       ${NC}"
echo -e "${GREEN}======================================================${NC}"
echo -e "Địa chỉ truy cập Panel : ${BLUE}http://${SERVER_IP}:${ACTIVE_PORT}${NC}"
echo -e "Tài khoản quản trị     : ${YELLOW}admin${NC}"
echo -e "Mật khẩu ngẫu nhiên    : ${YELLOW}${ADMIN_PASS}${NC}"
echo -e "Thư mục cài đặt        : ${INSTALL_DIR}"
echo -e "File cấu hình bảo mật  : ${INSTALL_DIR}/.env"
echo -e "${GREEN}======================================================${NC}"
echo -e "${YELLOW}LƯU Ý: Vui lòng lưu lại mật khẩu trên và cấu hình HTTPS cho panel!${NC}"
