# Pháp Lệnh Tối Cao Dự Án: VPS WordPress Control

> **Tài liệu Hiến Pháp Kỹ Thuật (Constitution)**  
> Mọi AI Agent và lập trình viên tham gia dự án bắt buộc phải tuân thủ nghiêm ngặt các nguyên tắc dưới đây.

---

## 1. TRIẾT LÝ CỐT LÕI (CORE PRINCIPLES)

1. **Lightweight & Dedicated First**:
   - Dự án được xây dựng riêng biệt cho WordPress trên VPS. Mọi tính năng phải giữ mức tiêu thụ tài nguyên tối thiểu (< 30MB RAM).
   - Ngôn ngữ chính Backend: **Golang** (Single static binary, Go Embed cho Frontend UI).
   - Cơ sở dữ liệu nội bộ: **SQLite (`wpcontrol.db`)**. Tuyệt đối không can thiệp, tạo bảng hay làm bẩn database MariaDB/MySQL của các website WordPress trên host.

2. **Nguyên Tắc Không Xâm Lấn (Non-Invasive Discovery)**:
   - Quá trình quét phát hiện website cũ (Auto-Discovery) luôn là chế độ **Read-Only**.
   - Không được tự ý sửa đổi file mã nguồn hay can thiệp vào database của site đang hoạt động trừ khi có thao tác lệnh tường minh từ người dùng.

3. **Phân Quyền File Linux An Toàn (Permission Safety)**:
   - Tự động nhận diện Linux user sở hữu website (`owner_user`) thay vì gán cứng `www-data` hoặc `root`.
   - Các tác vụ chỉnh sửa (update core, plugins, templates) phải đảm bảo khôi phục đúng quyền `chown` của user sở hữu thư mục web.

4. **1-Click Automation & Realtime Feedback**:
   - Tất cả các tác vụ nặng (quét mã độc, cập nhật core/plugins, đổi PHP, cài SSL) đều phải có cơ chế phản hồi trực quan trên Web UI qua **Progress Modal (SSE / Polling Fallback)**.
   - Luôn hỗ trợ script cập nhật 1 dòng lệnh (`update.sh`) không làm mất file cấu hình `.env` hoặc database.

---

## 2. QUY ĐỊNH BẢO MẬT (SECURITY FIRST)

- Không hard-code JWT Secret, mật khẩu admin trong mã nguồn. Toàn bộ đọc từ `.env`.
- Mọi API nội bộ (`/api/*`) đều bắt buộc xác thực qua Token HMAC-SHA256 (ngoại trừ `/api/health`, `/api/auth/login`, `/favicon.ico`).
- Hỗ trợ bộ lọc giới hạn IP (`ALLOWED_IPS`) theo từng IP hoặc dải mạng CIDR.
- Magic Login bắt buộc dùng One-Time Token tạm thời và tự xóa file `.wp_magic_*.php` ngay sau khi đăng nhập.

