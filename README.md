# VPS WordPress Control 🚀

> **Lightweight Dedicated Control Panel** dành riêng cho việc quản trị nhiều website WordPress trên VPS (Ubuntu / Debian).

![Go Version](https://img.shields.io/badge/Go-1.23-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/License-MIT-green.svg)
![Architecture](https://img.shields.io/badge/Architecture-amd64%20%7C%20arm64-blue)

---

## ✨ Tính Năng Nổi Bật

1. **Auto-Discovery Engine (Tự động quét):** Cài lên VPS đang chạy, tự động phát hiện toàn bộ website WordPress qua cấu hình Nginx (`/etc/nginx/sites-enabled/`) và thư mục web, đọc `wp-config.php`, phân tích plugin & theme mà không làm xáo trộn dữ liệu cũ.
2. **WordPress Centralized Manager:** Quản lý tập trung Plugin/Theme/Core, cập nhật hàng loạt 1-click, bật/tắt plugin trực tiếp từ Web UI để khắc phục màn hình trắng (WSOD).
3. **Magic Login (SSO 1-Click):** Tạo đường dẫn đăng nhập quản trị tạm thời, truy cập thẳng vào `wp-admin` mà không cần nhớ mật khẩu.
4. **Hệ Thống Quét Mã Độc Toàn Diện:**
   - **Lớp Checksum:** Xác thực tính toàn vẹn mã nguồn Core & Plugins với kho lưu trữ chính thức WordPress.org (`wp core verify-checksums` & `wp plugin verify-checksums`).
   - **Lớp Uploads:** Phát hiện và cảnh báo tức thì các file script thực thi PHP (`.php`, `.phtml`, `.phar`...) nằm trong thư mục `wp-content/uploads/`.
   - **Lớp Root Anomaly:** Nhận diện các file webshell phổ biến đặt ngoài thư mục gốc (`wso.php`, `c99.php`, `shell.php`...).
   - **Lớp Pattern Signature:** Quét regex chuyên sâu tìm backdoor, hàm mã hoá nguy hiểm (`eval(base64_decode)`, `gzinflate`, `assert`, `RCE`).
5. **Async Job Runner & Real-time Progress (SSE):** Quản lý các tác vụ tốn thời gian (quét toàn VPS, cập nhật hàng loạt website) với thanh tiến trình trực quan theo thời gian thực qua Server-Sent Events.
6. **Sao Lưu & Khôi Phục Nhanh (Backup Engine):** 1-click sao lưu toàn bộ Database & `wp-content` thành file `.tar.gz`, hỗ trợ khôi phục tức thì hoặc tải về.
7. **Tạo Website Mới 1-Click (Site Provisioning):** Tự động tạo thư mục web, tạo Database MariaDB, tải mã nguồn WordPress sạch theo phiên bản tùy chọn, tạo VirtualHost Nginx chuẩn bảo mật và cấp chứng chỉ Let's Encrypt SSL.
8. **Siêu Nhẹ & Bảo Mật Cao:** Viết bằng **Golang**, đóng gói trọn vẹn thành 1 file binary duy nhất (~15MB), tiêu thụ RAM < 30MB, hỗ trợ Whitelist IP (`ALLOWED_IPS`), token HMAC-SHA256, nhúng sẵn Web Dashboard giao diện TailwindCSS Dark Mode.

---

## ⚡ Cài Đặt Nhanh Trên VPS (1-Click Install)

Chạy lệnh sau trên VPS của bạn (yêu cầu quyền `root` trên Ubuntu 20.04/22.04/24.04 hoặc Debian 11/12):

```bash
curl -sSL https://raw.githubusercontent.com/acmavirus/vps-wordpress-control/main/install.sh | sudo bash
```

Sau khi cài đặt hoàn tất, script sẽ cung cấp:
* **Địa chỉ truy cập:** `http://<IP-VPS>:9090`
* **Tài khoản quản trị:** `admin`
* **Mật khẩu ngẫu nhiên:** Được sinh tự động và lưu an toàn tại `/opt/wp-control/.env`.

---

## 🔄 Cập Nhật Phiên Bản Mới (1-Click Update)

Khi có bản cập nhật mới, chỉ cần chạy đúng **1 dòng lệnh** duy nhất trên VPS:

```bash
curl -sSL https://raw.githubusercontent.com/acmavirus/vps-wordpress-control/main/update.sh | sudo bash
```
*(Script sẽ tự động tải binary mới nhất theo cấu trúc CPU của VPS, ghi đè và khởi động lại dịch vụ `wp-control` trong vòng 3 giây mà không làm mất cấu hình `.env` hay cơ sở dữ liệu)*

---

## 🛠 Quản Lý Dịch Vụ Trên VPS

```bash
# Kiểm tra trạng thái dịch vụ
sudo systemctl status wp-control

# Khởi động lại panel
sudo systemctl restart wp-control

# Xem log hoạt động theo thời gian thực
sudo journalctl -u wp-control -f
```

---

## 💻 Phát Triển Cục Bộ (Docker Development)

```bash
# 1. Khởi tạo môi trường
cp .env.example .env

# 2. Khởi chạy bằng Docker Compose
docker compose up -d

# 3. Truy cập Dashboard tại
http://localhost:9090
```

---

## 📄 Bản Quyền
Phát hành theo giấy phép MIT.
