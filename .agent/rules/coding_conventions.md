# Quy Ước Lập Trình (Coding Conventions) - VPS WordPress Control

---

## 1. MÃ NGUỒN BACKEND (GOLANG)

- **Go Version**: `1.22+` (Sử dụng enhanced routing của `http.NewServeMux` như `GET /api/sites/{id}`).
- **Formatting**: Luôn chạy `go fmt ./...` và kiểm tra cú pháp bằng `go vet ./...` trước khi commit.
- **Error Handling**: Không bao giờ bỏ qua lỗi (ignoring errors). Luôn trả về mã lỗi HTTP và JSON rõ ràng:
  ```json
  {"status": "error", "error": "Mô tả lỗi chi tiết"}
  ```
- **Concurrency**: Khi xử lý tác vụ nền (`goroutines`), luôn quản lý an toàn qua channel hoặc mutex trong `internal/jobs/runner.go`.

---

## 2. GIAO DIỆN WEB (FRONTEND UI)

- **Single Embedded File**: Toàn bộ giao diện nằm trong `internal/web/index.html` và được nhúng vào file nhị phân Go qua `internal/web/web.go`.
- **CSS Framework**: Sử dụng **TailwindCSS CDN** với bảng màu Dark Mode chuẩn (`bg-slate-900`, `bg-slate-800`, `text-slate-100`).
- **Icons**: FontAwesome 6 CDN (`fa-solid`, `fa-brands`).
- **Phản Hồi Trực Quan (UX/UI)**: Mọi thao tác chạy tác vụ (quét, cập nhật, đổi PHP, cài SSL) đều phải bật **Progress Modal** thông báo `%` tiến trình và trạng thái rõ ràng.

---

## 3. GIT & PHÁT HÀNH (RELEASES)

- Tuân thủ Semantic Versioning: `v1.0.x`.
- Mỗi lần release phiên bản mới:
  1. Chạy `go vet ./...` xác nhận 0 lỗi.
  2. Tạo commit Conventional Commits (`feat:`, `fix:`).
  3. Tạo git tag: `git tag v1.0.x`.
  4. Push nhánh main và tag: `git push origin main ; git push origin v1.0.x`.
  5. GitHub Actions sẽ tự động biên dịch nhị phân cho cả `amd64` và `arm64`.

