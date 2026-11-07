---
description: Quy trình debug và xử lý sự cố lỗi phát sinh trên Panel hoặc khi chạy lệnh WP-CLI/Nginx
globs: 
---

1. Thu thập thông tin lỗi từ log hoặc request người dùng:
   - Nếu là lỗi dịch vụ trên VPS: kiểm tra qua lệnh `sudo journalctl -u wp-control -n 100 -f`.
   - Nếu là lỗi HTTP: kiểm tra response code và chuỗi JSON báo lỗi trả về.
2. Xác định nguyên nhân gốc rễ (Root Cause):
   - Phân quyền Linux (`chown`, `chmod`).
   - Lỗi kết nối SQLite / Lock database.
   - Lỗi thực thi WP-CLI (`wp core`, `wp plugin`).
3. Sửa chữa tối thiểu (Minimal Atomic Diff), bảo toàn kiến trúc hiện tại.
4. Chạy kiểm tra tĩnh:
   ```powershell
   go vet ./...
   ```
5. Ghi lại nguyên nhân và giải pháp vào `.agent/memory/issue_resolutions.md`.

