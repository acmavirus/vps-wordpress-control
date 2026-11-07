# Nhật Ký Quyết Định Kỹ Thuật (Decision Log)

Ghi nhận các quyết định kiến trúc và công nghệ quan trọng của dự án `vps-wordpress-control`.

---

### [2026-11-07] Quyết Định 01: Lựa Chọn Golang + SQLite Cho Panel
- **Bối cảnh**: Cần một bảng điều khiển siêu nhẹ chạy nền trên VPS mà không làm tốn RAM của máy chủ (< 30MB).
- **Quyết định**: Sử dụng Golang nhúng toàn bộ HTML Frontend bằng `go:embed` và lưu trữ nội bộ bằng SQLite (`wpcontrol.db`).
- **Lợi ích**: 
  - Đóng gói ra 1 file binary duy nhất (~15MB), không phụ thuộc runtime Node.js/Python trên VPS.
  - SQLite chạy in-process, không phụ thuộc service MySQL của website.

---

### [2026-11-08] Quyết Định 02: Cơ Chế SSE + Polling Fallback Cho Hàng Đợi Tác Vụ
- **Bối cảnh**: Trình duyệt bị lỗi `ERR_INCOMPLETE_CHUNKED_ENCODING` khi kết nối Server-Sent Events bị ngắt hoặc do timeout mạng.
- **Quyết định**: 
  - Backend bỏ `WriteTimeout` cứng cho HTTP server.
  - Frontend tự động chuyển sang chế độ Polling kiểm tra `/api/jobs/{id}` khi luồng SSE đứt.
- **Lợi ích**: Tiến trình chạy ngầm (cập nhật hàng loạt, quét mã độc) luôn phản hồi mượt mà đến 100%.

---

### [2026-11-08] Quyết Định 03: One-Time Magic Login Tự Hủy
- **Bối cảnh**: Magic login 1-click trước đây fallback ra đường dẫn thư mục cục bộ của server gây 404 trên trình duyệt.
- **Quyết định**: Sinh file PHP tạm `.wp_magic_<token>.php` trong thư mục web, tự xóa (`unlink`) ngay sau khi đăng nhập và chuyển hướng vào `admin_url()`.
- **Lợi ích**: An toàn tuyệt đối, không lưu credentials và không để lại cửa sau.

---

### [2026-11-08] Quyết Định 04: UPSERT & Ràng Buộc UNIQUE Cho `site_plugins`
- **Bối cảnh**: Quét lại VPS nhiều lần gây trùng lặp danh sách plugin trên giao diện.
- **Quyết định**: Thêm ràng buộc `UNIQUE(site_id, slug)` và sử dụng cú pháp SQLite `ON CONFLICT(site_id, slug) DO UPDATE SET`.
- **Lợi ích**: Không bao giờ bị nhân bản plugin dù quét lại nhiều lần.

