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
4. **Quét Mã Độc 3 Lớp:**
   - Kiểm tra file thực thi PHP bất thường trong thư mục `wp-content/uploads/`.
   - Quét tìm các file khả nghi đặt ở thư mục gốc web.
   - Quét regex nhận diện các chuỗi backdoor / webshell nguy hiểm (`eval(base64_decode)`, `WSO`, `c99`, `assert`).
5. **Tạo Website Mới 1-Click (Site Provisioning):** Tự động tạo thư mục web, tạo Database MariaDB, tải mã nguồn WordPress sạch, tạo VirtualHost Nginx chuẩn bảo mật và cấp chứng chỉ Let's Encrypt SSL.
6. **Siêu Nhẹ & An Toàn:** Viết bằng **Golang**, đóng gói trọn vẹn thành 1 file binary duy nhất (~7MB), mức tiêu thụ RAM cực thấp (< 25MB), nhúng sẵn Web Dashboard giao diện hiện đại.

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
