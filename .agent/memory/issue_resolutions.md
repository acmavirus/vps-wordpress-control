# Nhật Ký Xử Lý Sự Cố (Issue Resolutions)

Lưu trữ các lỗi đã gặp và giải pháp chuẩn hóa để AI Agent tránh lặp lại.

---

### Sự cố 1: Permission Denied khi chạy `wp core update`
- **Hiện tượng**: `copy(.../wp-admin/includes/update-core.php): Permission denied in class-wp-filesystem-direct.php`.
- **Nguyên nhân**: Website nằm trong `/home/<domain>` thuộc quyền sở hữu của user riêng, nhưng lệnh wp-cli trước đó chạy dưới user `www-data` hoặc file bị root chiếm quyền.
- **Giải pháp**:
  - `DiscoveryEngine` tự động đọc UID/owner của `wp-config.php` trên Linux lưu vào `owner_user`.
  - Trong `wpcli.Run`: Nếu chạy dưới quyền user bị lỗi permission, tự động fallback chạy dưới quyền root (`--allow-root`) để ghi đè file thành công, sau đó tự động gọi `chown -R <owner>:<owner>` khôi phục quyền chuẩn.

---

### Sự cố 2: `ERR_INCOMPLETE_CHUNKED_ENCODING` trên SSE
- **Hiện tượng**: Khi gọi API stream SSE `/api/jobs/{id}/stream`, stream bị ngắt sau 15 giây.
- **Nguyên nhân**: Go `http.Server` cấu hình `WriteTimeout = 15s`.
- **Giải pháp**:
  - Đặt `WriteTimeout = 0` và `ReadTimeout = 0` cho streaming.
  - Thêm header `X-Accel-Buffering: no`.
  - Frontend bổ sung handler `es.onerror` tự động fallback sang polling `GET /api/jobs/{id}` định kỳ 1.5s/lần.

---

### Sự cố 3: GitHub Releases CDN Cache khiến lệnh `update.sh` tải lại bản cũ
- **Hiện tượng**: Đã tag bản mới nhưng chạy `update.sh` vẫn bị bản cũ do link `releases/latest` bị cache trên CDN Fastly 5-10 phút.
- **Giải pháp**: Trong `update.sh`, truy vấn trực tiếp GitHub REST API để lấy chính xác Tag mới nhất rồi tải theo đường dẫn direct `releases/download/<TAG>/...`.
