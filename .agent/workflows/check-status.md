---
description: Kiểm tra toàn diện trạng thái hệ thống, database SQLite và tính toàn vẹn của mã nguồn Go
globs: 
---

1. Kiểm tra mã nguồn bằng lệnh:
   ```powershell
   go vet ./...
   ```
2. Kiểm tra trạng thái Git và các commit gần nhất:
   ```powershell
   git status --short
   ```
3. Kiểm tra các port và biến môi trường trong file `.env`:
   - Xác nhận `APP_PORT` nằm trong dải `9000-9999`.
   - Xác nhận `DATA_DIR` và `DB_NAME` hợp lệ.
4. Báo cáo trạng thái sẵn sàng của dự án.
