---
name: vps-wordpress-control-guide
description: Hướng dẫn chuyên sâu và quy tắc phát triển dành riêng cho bảng điều khiển VPS WordPress Control
---

# VPS WordPress Control Guide & Standards

Tài liệu hướng dẫn phát triển và mở rộng tính năng cho dự án.

## 1. Cấu Trúc Thư Mục Chuẩn
- `cmd/server/main.go`: Điểm khởi chạy HTTP Server, khởi tạo Router, nhúng UI và kích hoạt Graceful Shutdown.
- `internal/api/`: Các HTTP Handlers tiếp nhận request API (Auth, Site, WP, Malware, Backup, Job).
- `internal/jobs/`: Engine xử lý tác vụ nền (`Runner`) và phát luồng SSE (`JobEvent`).
- `internal/scanner/`: Bộ quét tự động tìm website (`DiscoveryEngine`) và quét mã độc 4 lớp (`MalwareScanner`).
- `internal/wpcli/`: Wrapper thực thi các lệnh WP-CLI an toàn với phân quyền Linux.
- `internal/backup/`: Engine sao lưu nén Database + wp-content thành file `.tar.gz`.
- `internal/web/`: File HTML SPA nhúng trực tiếp qua Go Embed.

## 2. Các Quy Tắc Bắt Buộc
- **Không bao giờ làm mất dữ liệu người dùng khi cập nhật**: Tất cả các cập nhật qua `update.sh` chỉ thay thế file thực thi `bin/wp-control`, tuyệt đối không ghi đè `data/` hay `.env`.
- **Luôn có Progress Modal**: Mọi tác vụ có độ trễ > 1s phải có phản hồi thị giác trên Dashboard.
