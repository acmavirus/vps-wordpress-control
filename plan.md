# Kế Hoạch Phát Triển Dự Án: VPS WordPress Control (plan.md)

> **Dự án:** `vps-wordpress-control`  
> **Mô hình:** Lightweight Self-Hosted Control Panel chuyên biệt cho WordPress trên VPS  
> **Mục tiêu:** Quản trị toàn diện vòng đời website WordPress trên VPS: Tự động quét nhận diện site có sẵn, quản lý plugin/theme/core tập trung, quét mã độc 3 lớp, tạo site mới tối ưu Nginx + SSL 1-click.

---

## 1. TỔNG QUAN VÀ ĐỊNH VỊ SẢN PHẨM

### 1.1. Tầm nhìn
Khắc phục nhược điểm của các Server Panel truyền thống (quá cồng kềnh, thiếu tính năng chuyên sâu cho WordPress) và các công cụ quản lý WP dạng SaaS (không can thiệp được vào hạ tầng Nginx/Vhost/SSL của VPS). `vps-wordpress-control` là bảng điều khiển siêu nhẹ, cài trực tiếp lên VPS, chạy ngầm với tài nguyên cực thấp (< 30MB RAM), tập trung 100% vào WordPress.

### 1.2. Tính năng Cốt lõi (Core Features)
1. **Auto-Discovery Engine:** Tự động quét toàn bộ VPS, phát hiện các website WordPress có sẵn thông qua Nginx Vhost và thư mục hệ thống mà không làm ảnh hưởng đến cấu hình cũ.
2. **WordPress Centralized Manager:** Quản lý tập trung Plugin, Theme, Core; cập nhật hàng loạt (bulk update); kích hoạt/vô hiệu hóa plugin chống màn hình trắng (WSOD); đăng nhập 1-click vào wp-admin (Magic Login / SSO).
3. **Tạo Website Mới (1-Click Provisioning):** Tạo VirtualHost Nginx tối ưu cho WP, tạo Database MariaDB, tải mã nguồn sạch qua WP-CLI, cấp chứng chỉ SSL Let's Encrypt tự động.
4. **Hệ thống Quét Mã Độc 3 Lớp:** Kiểm tra tính toàn vẹn (Checksums core & plugins), phát hiện file PHP lạ trong thư mục `wp-content/uploads/`, quét regex các pattern backdoor/webshell nguy hiểm.
5. **Tác Vụ Bất Đồng Bộ (Job Runner):** Xử lý các tác vụ nặng (quét mã độc, cập nhật nhiều site) dưới nền và báo cáo tiến độ thời gian thực (Real-time SSE).

---

## 2. KIẾN TRÚC KỸ THUẬT VÀ TECH STACK

* **Backend & System Service:** **Golang** (Compile single binary, RAM ~15-25MB).
* **Cơ sở dữ liệu nội bộ Panel:** **SQLite (`wpcontrol.db`)**.
* **Frontend UI:** **Vue.js / React + TailwindCSS** (Nhúng tĩnh vào Go binary).
* **Cổng dịch vụ:** `9090` (Dải port 9000-9999).
* **Cơ chế Phân phối:** Binary qua GitHub Releases, cài đặt 1 dòng lệnh qua `install.sh`.

