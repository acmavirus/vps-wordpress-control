# Kế Hoạch Phát Triển Dự Án: VPS WordPress Control (plan.md)

> **Dự án:** `vps-wordpress-control`  
> **Mô hình:** Lightweight Self-Hosted Control Panel chuyên biệt cho WordPress trên VPS  
> **Mục tiêu:** Quản trị toàn diện vòng đời website WordPress trên VPS: Tự động quét nhận diện site có sẵn, quản lý plugin/theme/core tập trung, quét mã độc 3 lớp, tạo site mới tối ưu Nginx + SSL 1-click.
> Github: https://github.com/acmavirus/vps-wordpress-control

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

### 2.1. Tech Stack Lựa Chọn

- **Panel Backend & System Service:** **Golang** (hoặc Node.js Containerized).
  - Compile ra 1 file thực thi nhị phân (single binary), không phụ thuộc môi trường host.
  - Mức tiêu hao RAM tối thiểu (~15MB – 30MB).
  - Gọi trực tiếp các lệnh hệ thống Linux (Nginx, Certbot, WP-CLI) với hiệu năng cao.
- **Cơ sở dữ liệu nội bộ Panel:** **SQLite (`wpcontrol.db`)**
  - Tự lưu trữ cấu hình, danh sách site, cache plugin và lịch sử quét.
  - Không làm phân tán hoặc phụ thuộc vào MySQL/MariaDB của website.
- **Panel Frontend UI:** **Vue.js / Svelte / React + TailwindCSS**
  - Giao diện SPA tinh gọn, hiện đại, hỗ trợ Dark/Light mode.
  - Đóng gói trực tiếp vào binary của Backend (Go Embed) phục vụ trên cổng riêng.
- **Cổng dịch vụ (Port):** Mặc định sử dụng cổng trong dải **9000-9999** (ví dụ: `9443` hoặc lấy từ file cấu hình `.env`).
- **Công cụ bổ trợ tầng OS:** `Nginx`, `PHP-FPM` (hỗ trợ đa phiên bản 7.4 - 8.3), `MariaDB/MySQL`, `WP-CLI`, `Certbot`.

### 2.2. Kiến trúc Thư mục trên VPS

```
/opt/wp-control/
├── bin/
│   └── wp-control                 # Binary điều khiển chính
├── data/
│   └── wpcontrol.db               # SQLite database
├── templates/
│   ├── nginx-wp-standard.conf     # Template Nginx chuẩn
│   └── nginx-wp-cache.conf        # Template Nginx có FastCGI Cache
├── scripts/
│   └── magic-login.php            # Script tạo URL đăng nhập 1-click
└── logs/
    └── wp-control.log             # Log hoạt động hệ thống
```

---

## 3. THIẾT KẾ CƠ SỞ DỮ LIỆU NỘI BỘ (SQLITE SCHEMA)

```sql
-- 1. Bảng lưu trữ website
CREATE TABLE sites (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    domain TEXT UNIQUE NOT NULL,
    root_path TEXT NOT NULL,
    php_version TEXT DEFAULT '8.2',
    wp_version TEXT,
    owner_user TEXT DEFAULT 'www-data',
    db_name TEXT,
    db_user TEXT,
    ssl_active INTEGER DEFAULT 0,
    ssl_expires_at DATETIME,
    fastcgi_cache INTEGER DEFAULT 0,
    is_discovered INTEGER DEFAULT 0,
    status TEXT DEFAULT 'active',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 2. Bảng cache plugin & theme
CREATE TABLE site_plugins (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    site_id INTEGER NOT NULL,
    slug TEXT NOT NULL,
    name TEXT,
    version TEXT,
    update_version TEXT,
    status TEXT,
    has_update INTEGER DEFAULT 0,
    FOREIGN KEY(site_id) REFERENCES sites(id) ON DELETE CASCADE
);

-- 3. Bảng kết quả quét mã độc
CREATE TABLE scan_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    site_id INTEGER NOT NULL,
    scan_type TEXT,                  -- checksum | upload_php | malware_pattern
    file_path TEXT NOT NULL,
    issue_description TEXT,
    severity TEXT DEFAULT 'warning', -- info | warning | critical
    detected_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(site_id) REFERENCES sites(id) ON DELETE CASCADE
);

-- 4. Bảng quản lý hàng đợi tác vụ
CREATE TABLE jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    site_id INTEGER,
    action TEXT NOT NULL,            -- create_site | update_plugins | scan_malware
    status TEXT DEFAULT 'pending',   -- pending | running | completed | failed
    progress INTEGER DEFAULT 0,
    log_output TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

---

## 4. CHI TIẾT CÁC PHÂN HỆ XỬ LÝ (MODULES SPECIFICATION)

### Module 1: Auto-Discovery Engine (Quét & Nhận Diện Site Cũ)

- **Luồng xử lý:**
  1. Quét toàn bộ file cấu hình trong `/etc/nginx/sites-enabled/*` hoặc `/etc/nginx/conf.d/*.conf`.
  2. Dùng Regex trích xuất `server_name` (domain) và `root` (thư mục chứa web).
  3. Quét dự phòng các đường dẫn tiêu chuẩn: `/var/www/*`, `/home/*/public_html`.
  4. Kiểm tra sự tồn tại của `wp-config.php`.
  5. Đọc cấu hình database (`DB_NAME`, `DB_USER`, `DB_PASSWORD`, `$table_prefix`).
  6. Thực thi lệnh WP-CLI dưới quyền user sở hữu thư mục (ví dụ `sudo -u www-data wp core version --path=...`):
     - Thu thập phiên bản WordPress.
     - Thu thập danh sách plugin & theme kèm trạng thái update.
  7. Đồng bộ dữ liệu vào bảng `sites` và `site_plugins` với cờ `is_discovered = 1`.

### Module 2: WordPress App Manager (Quản Trị Tầng Ứng Dụng)

- **Cập nhật hàng loạt (Bulk Updates):**
  - Hỗ trợ cập nhật từng plugin hoặc cập nhật tất cả plugin/theme/core của 1 site hoặc tất cả các site trên VPS.
  - Thực thi thông qua `wp plugin update <slug>` và `wp core update`.
- **Khắc phục sự cố xung đột (WSOD Fixer):**
  - Liệt kê plugin đang kích hoạt. Cho phép bật/tắt trực tiếp từ Dashboard để xử lý lỗi trắng trang khi plugin bị xung đột sau khi cập nhật.
- **Magic Login (SSO 1-Click):**
  - Sinh đường dẫn đăng nhập quản trị tạm thời có thời hạn (URL Token) để quản trị viên truy cập thẳng vào `wp-admin` mà không cần nhập mật khẩu.

### Module 3: Malware & Security Scanner (Quét Mã Độc 3 Lớp)

- **Lớp 1 - Checksum Integrity:**
  - Chạy `wp core verify-checksums` và `wp plugin verify-checksums --all`.
  - So sánh hash của file core và file plugin trên VPS với kho lưu trữ chính thức của WordPress.org.
- **Lớp 2 - Uploads Anomaly Detection:**
  - Quét toàn bộ thư mục `wp-content/uploads/`.
  - Gắn cờ cảnh báo **CRITICAL** ngay lập tức nếu xuất hiện bất kỳ file nào có phần mở rộng `.php`, `.phtml`, `.php5`.
- **Lớp 3 - Pattern & Signature Scanner:**
  - Quét regex tìm kiếm các chuỗi mã hóa/hàm thực thi nguy hiểm thường dùng trong webshell (như `eval(base64_decode`, `gzinflate`, `assert`, `shell_exec`, `c99`, `r57`).

### Module 4: Site Provisioning (Tạo Site Mới 1-Click)

- **Quy trình tự động hóa:**
  1. Tiếp nhận thông tin từ form: `Domain`, `Tên Website`, `Admin Email`, `PHP Version`.
  2. Tạo thư mục `/var/www/<domain>/public_html` và cấp quyền sở hữu `www-data:www-data` (chmod 755/644).
  3. Tạo database và user database ngẫu nhiên trên MariaDB.
  4. Tải mã nguồn WordPress sạch bằng WP-CLI (`wp core download`).
  5. Sinh file `wp-config.php` với cấu hình bảo mật chuẩn (khóa salt ngẫu nhiên).
  6. Cài đặt hoàn thiện (`wp core install`).
  7. Tạo cấu hình VirtualHost Nginx từ template chuẩn (chặn truy cập wp-config, chặn php trong uploads, chặn xmlrpc).
  8. Kiểm tra cấu hình Nginx (`nginx -t`) và reload dịch vụ.
  9. Yêu cầu cấp chứng chỉ Let's Encrypt qua `certbot` và kích hoạt HTTPS.

### Module 5: Async Task Runner & Realtime Progress

- Quản lý các tác vụ tốn thời gian (quét mã độc nhiều site, cập nhật đồng loạt).
- Ghi log từng bước vào bảng `jobs`.
- Phát luồng sự kiện qua **Server-Sent Events (SSE)** hoặc WebSocket để cập nhật thanh % tiến trình trực quan trên giao diện người dùng.

---

## 5. LỘ TRÌNH TRIỂN KHAI THEO TỪNG GIAI ĐOẠN (ROADMAP)

### Giai đoạn 1: Khởi tạo Kiến trúc & Môi trường Phát triển (Sprint 1)

- [ ] Thiết lập cấu trúc thư mục dự án (`vps-wordpress-control`).
- [ ] Tạo file cấu hình môi trường `.env.example` (cổng dịch vụ, đường dẫn cấu hình).
- [ ] Khởi tạo dự án Backend (Golang hoặc Node.js Containerized) kết nối SQLite.
- [ ] Tạo bảng SQLite theo schema đã thiết kế.

### Giai đoạn 2: Xây Dựng Core Engine (CLI & Auto-Discovery) (Sprint 2)

- [ ] Xây dựng bộ parser đọc file cấu hình Nginx (`/etc/nginx/`).
- [ ] Xây dựng thuật toán nhận diện WordPress và đọc `wp-config.php`.
- [ ] Xây dựng Wrapper gọi lệnh `WP-CLI` an toàn dưới quyền user hệ thống.
- [ ] Viết chức năng quét tự động và đồng bộ danh sách site vào SQLite.

### Giai đoạn 3: Phân Hệ Tạo Site Mới & Quản Lý Nginx/SSL (Sprint 3)

- [ ] Tạo template cấu hình Nginx bảo mật cho WordPress.
- [ ] Xây dựng pipeline tạo Database MariaDB tự động.
- [ ] Tích hợp `wp core download` và `wp core install`.
- [ ] Tích hợp Certbot tự động cấp chứng chỉ SSL Let's Encrypt.

### Giai đoạn 4: Phân Hệ Quản Trị Ứng Dụng & Quét Mã Độc (Sprint 4)

- [ ] Xây dựng API liệt kê và cập nhật Plugin/Theme/Core (Bulk Update).
- [ ] Xây dựng chức năng bật/tắt plugin để xử lý xung đột.
- [ ] Xây dựng script Magic Login 1-click vào `wp-admin`.
- [ ] Hiện thực hóa 3 lớp quét mã độc (Checksum, Uploads scan, Signature scan).

### Giai đoạn 5: Xây Dựng Giao Diện Dashboard (Web UI) (Sprint 5)

- [ ] Thiết kế Layout Dashboard hiện đại với TailwindCSS.
- [ ] Trang Quản lý Website: Danh sách site, phiên bản WP, trạng thái SSL.
- [ ] Trang Quản lý Plugin: Liệt kê plugin của từng site hoặc toàn server, nút cập nhật hàng loạt.
- [ ] Trang Bảo mật & Quét Mã Độc: Nút kích hoạt quét, bảng báo cáo file bất thường.
- [ ] Modal Thêm Site Mới 1-Click.
- [ ] Tích hợp Real-time Progress Bar qua SSE.

### Giai đoạn 6: Đóng Gói, Script Cài Đặt 1-Click & Kiểm Thử (Sprint 6)

- [ ] Viết script cài đặt tự động 1 dòng lệnh: `install.sh`.
- [ ] Thiết lập Systemd Service để panel tự khởi động cùng VPS.
- [ ] Kiểm thử toàn diện trên môi trường VPS thực tế (Ubuntu 22.04 / 24.04).
- [ ] Hoàn thiện tài liệu hướng dẫn sử dụng `README.md`.

---

## 6. QUY TẮC AN TOÀN & BẢO MẬT HỆ THỐNG

1. **Nguyên tắc "Không Xâm Lấn" Site Cũ:**
   - Quá trình Auto-Discovery chỉ **đọc** cấu hình (`read-only`), tuyệt đối không chỉnh sửa file mã nguồn hay can thiệp vào database của website đang hoạt động trừ khi có lệnh thao tác tường minh từ quản trị viên.
2. **Nguyên tắc Phân Quyền File Linux:**
   - Luôn đảm bảo file mới tạo thuộc quyền sở hữu của `www-data` (hoặc user quản trị tương ứng).
   - Không chạy WP-CLI với cờ `--allow-root` trực tiếp làm thay đổi owner của file sang `root`.
3. **Bảo Mật Panel:**
   - Giao diện Panel bắt buộc phải có tài khoản xác thực (Username/Password có băm bcrypt).
   - Hỗ trợ giới hạn IP truy cập hoặc cổng riêng biệt bảo mật qua SSL.
