---
description: Quy trình kiểm tra chất lượng mã nguồn trước khi commit hoặc tạo Release mới
globs: 
---

1. Kiểm tra tuân thủ nguyên tắc không hard-code credentials.
2. Kiểm tra `go vet ./...` không có bất kỳ warning hoặc syntax error nào.
3. Kiểm tra tính toàn vẹn của file nhúng `internal/web/index.html`.
4. Xác nhận các thay đổi không gây breaking changes đối với schema SQLite hoặc script cài đặt `install.sh` / `update.sh`.
5. Đảm bảo UI tuân thủ nguyên tắc: văn bản tiêu chuẩn, tối giản, có progress modal đầy đủ.
