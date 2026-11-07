# Chính Sách Bảo Mật (Security Policy)

---

## 1. BẢO MẬT HỆ THỐNG PANEL
- **Authentication**: Bắt buộc có token HMAC-SHA256 (`Bearer <token>`) với hạn dùng 7 ngày.
- **Port**: Cổng dịch vụ mặc định `9090` (thuộc dải 9000-9999 theo quy ước chuẩn).
- **IP Restriction**: Hỗ trợ cấu hình `ALLOWED_IPS` trong `.env` để chỉ cho phép các IP đáng tin cậy truy cập vào panel.
- **Tách Biệt Dữ Liệu**: Database nội bộ SQLite (`wpcontrol.db`) độc lập hoàn toàn với database MariaDB của các site khách hàng.

---

## 2. BẢO MẬT WEBSITE WORDPRESS TRÊN VPS
- **Nginx Template Chuẩn**:
  - Chặn truy cập trực tiếp file cấu hình nhạy cảm `wp-config.php`.
  - Chặn thực thi mã PHP trong thư mục `wp-content/uploads/`.
  - Tắt XML-RPC chống tấn công DDoS brute-force.
- **Hệ Thống Quét Mã Độc 4 Tầng**:
  1. Checksum Core & Plugins qua WP-CLI (đối chiếu hash chính thức từ WordPress.org).
  2. Phát hiện file PHP/executable lạ trong thư mục `uploads`.
  3. Quét file tên webshell phổ biến ở thư mục gốc (`wso.php`, `c99.php`).
  4. Quét regex pattern backdoor/webshell nguy hiểm (`eval(base64_decode)`, `gzinflate`, `assert`).
- **SSL Let's Encrypt**: Tự động cấu hình Certbot chuyển hướng toàn bộ traffic sang HTTPS.

