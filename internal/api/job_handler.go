package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"vps-wordpress-control/internal/jobs"
)

// JobHandler xử lý API tạo job và SSE stream tiến trình
type JobHandler struct {
	db     *sql.DB
	runner *jobs.Runner
}

func NewJobHandler(db *sql.DB, runner *jobs.Runner) *JobHandler {
	return &JobHandler{db: db, runner: runner}
}

func (h *JobHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/jobs/scan-all", h.HandleStartScanAll)
	mux.HandleFunc("POST /api/jobs/bulk-update", h.HandleStartBulkUpdate)
	mux.HandleFunc("GET /api/jobs/{id}", h.HandleGetJob)
	mux.HandleFunc("GET /api/jobs/{id}/stream", h.HandleJobStream)
	mux.HandleFunc("GET /api/jobs", h.HandleListJobs)
}

// HandleStartScanAll tạo và chạy job quét mã độc toàn bộ VPS
func (h *JobHandler) HandleStartScanAll(w http.ResponseWriter, r *http.Request) {
	jobID, err := h.runner.CreateJob(nil, "scan_all")
	if err != nil {
		http.Error(w, `{"error": "Không thể tạo job"}`, http.StatusInternalServerError)
		return
	}

	go h.runner.RunScanAll(jobID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "started",
		"job_id": jobID,
	})
}

// HandleStartBulkUpdate tạo và chạy job cập nhật plugin hàng loạt
func (h *JobHandler) HandleStartBulkUpdate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SiteIDs []int64 `json:"site_ids"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	jobID, err := h.runner.CreateJob(nil, "bulk_update")
	if err != nil {
		http.Error(w, `{"error": "Không thể tạo job"}`, http.StatusInternalServerError)
		return
	}

	go h.runner.RunBulkUpdate(jobID, body.SiteIDs)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "started",
		"job_id": jobID,
	})
}

// HandleGetJob trả về trạng thái của một job
func (h *JobHandler) HandleGetJob(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	jobID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "Job ID không hợp lệ"}`, http.StatusBadRequest)
		return
	}

	info, err := h.runner.GetJob(jobID)
	if err == sql.ErrNoRows {
		http.Error(w, `{"error": "Job không tồn tại"}`, http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, `{"error": "Lỗi truy vấn job"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"job":    info,
	})
}

// HandleJobStream phát luồng SSE cập nhật tiến trình job theo thời gian thực
func (h *JobHandler) HandleJobStream(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	jobID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Job ID không hợp lệ", http.StatusBadRequest)
		return
	}

	// Thiết lập header SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE không được hỗ trợ", http.StatusInternalServerError)
		return
	}

	ch := h.runner.Subscribe(jobID)
	defer h.runner.Unsubscribe(jobID, ch)

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case event, open := <-ch:
			if !open {
				return
			}
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			// Đóng stream khi job hoàn thành hoặc thất bại
			if event.Status == "completed" || event.Status == "failed" {
				return
			}

		case <-ticker.C:
			// Keep-alive comment để giữ connection
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()

		case <-r.Context().Done():
			return
		}
	}
}

// HandleListJobs liệt kê 20 job gần nhất
func (h *JobHandler) HandleListJobs(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`
		SELECT id, site_id, action, status, progress, log_output, created_at 
		FROM jobs 
		ORDER BY id DESC 
		LIMIT 20
	`)
	if err != nil {
		http.Error(w, `{"error": "Lỗi truy vấn"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var list []map[string]interface{}
	for rows.Next() {
		var id, progress int64
		var siteID sql.NullInt64
		var action, status, createdAt string
		var logOutput sql.NullString

		if err := rows.Scan(&id, &siteID, &action, &status, &progress, &logOutput, &createdAt); err == nil {
			list = append(list, map[string]interface{}{
				"id":         id,
				"site_id":    siteID.Int64,
				"action":     action,
				"status":     status,
				"progress":   progress,
				"log_output": logOutput.String,
				"created_at": createdAt,
			})
		}
	}

	if list == nil {
		list = []map[string]interface{}{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"jobs":   list,
		"count":  len(list),
	})
}

