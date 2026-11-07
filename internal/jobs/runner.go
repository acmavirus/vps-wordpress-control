package jobs

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"vps-wordpress-control/internal/scanner"
	"vps-wordpress-control/internal/wpcli"
)

// JobEvent là sự kiện được phát qua SSE
type JobEvent struct {
	JobID    int64  `json:"job_id"`
	Status   string `json:"status"`
	Progress int    `json:"progress"`
	Message  string `json:"message"`
}

// Runner quản lý hàng đợi tác vụ bất đồng bộ và SSE broadcast
type Runner struct {
	db      *sql.DB
	wpcli   *wpcli.WPCLI
	scanner *scanner.MalwareScanner

	mu      sync.RWMutex
	streams map[int64][]chan JobEvent
}

func NewRunner(db *sql.DB) *Runner {
	return &Runner{
		db:      db,
		wpcli:   wpcli.NewWPCLI(),
		scanner: scanner.NewMalwareScanner(db),
		streams: make(map[int64][]chan JobEvent),
	}
}

// Subscribe đăng ký một SSE client để nhận update của job
func (r *Runner) Subscribe(jobID int64) chan JobEvent {
	ch := make(chan JobEvent, 20)
	r.mu.Lock()
	r.streams[jobID] = append(r.streams[jobID], ch)
	r.mu.Unlock()

	// Gửi trạng thái hiện tại ngay lập tức khi subscribe
	go func() {
		info, err := r.GetJob(jobID)
		if err == nil {
			ch <- JobEvent{
				JobID:    jobID,
				Status:   fmt.Sprintf("%v", info["status"]),
				Progress: int(info["progress"].(int64)),
				Message:  fmt.Sprintf("%v", info["log_output"]),
			}
		}
	}()

	return ch
}

// Unsubscribe hủy đăng ký SSE client
func (r *Runner) Unsubscribe(jobID int64, ch chan JobEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	list := r.streams[jobID]
	for i, c := range list {
		if c == ch {
			r.streams[jobID] = append(list[:i], list[i+1:]...)
			close(ch)
			return
		}
	}
}

// broadcast phát sự kiện đến tất cả subscribers của một job
func (r *Runner) broadcast(event JobEvent) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, ch := range r.streams[event.JobID] {
		select {
		case ch <- event:
		default:
		}
	}
}

// CreateJob tạo một bản ghi job mới trong database
func (r *Runner) CreateJob(siteID *int64, action string) (int64, error) {
	result, err := r.db.Exec(
		"INSERT INTO jobs (site_id, action, status, progress, log_output) VALUES (?, ?, 'pending', 0, ?)",
		siteID, action, "Đang chờ xử lý...",
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// GetJob trả về thông tin của một job
func (r *Runner) GetJob(jobID int64) (map[string]interface{}, error) {
	var id, progress int64
	var siteID sql.NullInt64
	var action, status string
	var logOutput sql.NullString
	var createdAt string

	err := r.db.QueryRow(
		"SELECT id, site_id, action, status, progress, log_output, created_at FROM jobs WHERE id = ?",
		jobID,
	).Scan(&id, &siteID, &action, &status, &progress, &logOutput, &createdAt)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"id":         id,
		"site_id":    siteID.Int64,
		"action":     action,
		"status":     status,
		"progress":   progress,
		"log_output": logOutput.String,
		"created_at": createdAt,
	}, nil
}

// RunScanAll quét mã độc tất cả website theo kiểu bất đồng bộ
func (r *Runner) RunScanAll(jobID int64) {
	r.updateJob(jobID, "running", 0, "Đang lấy danh sách website...")

	rows, err := r.db.Query("SELECT id, domain, root_path FROM sites WHERE status = 'active'")
	if err != nil {
		r.updateJob(jobID, "failed", 0, fmt.Sprintf("Lỗi truy vấn danh sách website: %v", err))
		return
	}
	defer rows.Close()

	type siteInfo struct {
		ID       int64
		Domain   string
		RootPath string
	}

	var sites []siteInfo
	for rows.Next() {
		var s siteInfo
		if err := rows.Scan(&s.ID, &s.Domain, &s.RootPath); err == nil {
			sites = append(sites, s)
		}
	}

	total := len(sites)
	if total == 0 {
		r.updateJob(jobID, "completed", 100, "Không có website nào để quét.")
		return
	}

	log.Printf("[Job Runner] Job #%d: Bắt đầu quét %d website...", jobID, total)
	totalWarnings := 0

	for i, site := range sites {
		progress := int(float64(i) / float64(total) * 100)
		msg := fmt.Sprintf("Đang quét [%d/%d]: %s", i+1, total, site.Domain)
		r.updateJob(jobID, "running", progress, msg)

		findings, err := r.scanner.ScanSite(site.ID, site.RootPath)
		if err != nil {
			log.Printf("[Job Runner] Lỗi quét %s: %v", site.Domain, err)
			continue
		}
		totalWarnings += len(findings)
	}

	finalMsg := fmt.Sprintf("Hoàn tất quét %d website! Tổng cảnh báo: %d", total, totalWarnings)
	r.updateJob(jobID, "completed", 100, finalMsg)
	log.Printf("[Job Runner] Job #%d hoàn tất: %s", jobID, finalMsg)
}

// RunBulkUpdate cập nhật plugins hàng loạt theo kiểu bất đồng bộ
func (r *Runner) RunBulkUpdate(jobID int64, siteIDs []int64) {
	r.updateJob(jobID, "running", 0, "Đang chuẩn bị cập nhật...")

	var query string
	var rows *sql.Rows
	var err error

	if len(siteIDs) > 0 {
		placeholders := make([]byte, 0)
		args := make([]interface{}, len(siteIDs))
		for i, id := range siteIDs {
			if i > 0 {
				placeholders = append(placeholders, ',')
			}
			placeholders = append(placeholders, '?')
			args[i] = id
		}
		query = fmt.Sprintf("SELECT id, domain, root_path, owner_user FROM sites WHERE id IN (%s)", string(placeholders))
		rows, err = r.db.Query(query, args...)
	} else {
		query = `SELECT DISTINCT s.id, s.domain, s.root_path, s.owner_user 
				 FROM sites s INNER JOIN site_plugins p ON s.id = p.site_id
				 WHERE p.has_update = 1 AND s.status = 'active'`
		rows, err = r.db.Query(query)
	}

	if err != nil {
		r.updateJob(jobID, "failed", 0, fmt.Sprintf("Lỗi truy vấn: %v", err))
		return
	}
	defer rows.Close()

	type siteInfo struct {
		ID        int64
		Domain    string
		RootPath  string
		OwnerUser string
	}

	var sites []siteInfo
	for rows.Next() {
		var s siteInfo
		if err := rows.Scan(&s.ID, &s.Domain, &s.RootPath, &s.OwnerUser); err == nil {
			sites = append(sites, s)
		}
	}

	total := len(sites)
	if total == 0 {
		r.updateJob(jobID, "completed", 100, "Không có website nào cần cập nhật plugin.")
		return
	}

	successCount := 0
	type resultItem struct {
		Domain  string `json:"domain"`
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}
	var results []resultItem

	for i, site := range sites {
		progress := int(float64(i) / float64(total) * 100)
		msg := fmt.Sprintf("Đang cập nhật [%d/%d]: %s", i+1, total, site.Domain)
		r.updateJob(jobID, "running", progress, msg)

		_, err := r.wpcli.Run(site.RootPath, site.OwnerUser, "plugin", "update", "--all")
		if err != nil {
			results = append(results, resultItem{Domain: site.Domain, Success: false, Error: err.Error()})
		} else {
			_, _ = r.db.Exec("UPDATE site_plugins SET has_update = 0 WHERE site_id = ?", site.ID)
			results = append(results, resultItem{Domain: site.Domain, Success: true})
			successCount++
		}
	}

	summaryBytes, _ := json.Marshal(results)
	finalMsg := fmt.Sprintf("Hoàn tất! Cập nhật thành công %d/%d website. Chi tiết: %s", successCount, total, string(summaryBytes))
	r.updateJob(jobID, "completed", 100, finalMsg)
	log.Printf("[Job Runner] Job #%d hoàn tất: %d/%d sites updated", jobID, successCount, total)
}

// updateJob cập nhật trạng thái job trong DB và broadcast SSE
func (r *Runner) updateJob(jobID int64, status string, progress int, message string) {
	_, _ = r.db.Exec(
		"UPDATE jobs SET status = ?, progress = ?, log_output = ? WHERE id = ?",
		status, progress, message, jobID,
	)
	r.broadcast(JobEvent{
		JobID:    jobID,
		Status:   status,
		Progress: progress,
		Message:  message,
	})
}

