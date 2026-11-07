package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"vps-wordpress-control/internal/backup"
	"vps-wordpress-control/internal/config"
)

type BackupHandler struct {
	engine *backup.BackupEngine
}

func NewBackupHandler(cfg *config.Config, db *sql.DB) *BackupHandler {
	return &BackupHandler{
		engine: backup.NewBackupEngine(cfg, db),
	}
}

func (h *BackupHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/sites/{id}/backups", h.HandleCreateBackup)
	mux.HandleFunc("GET /api/sites/{id}/backups", h.HandleListBackups)
	mux.HandleFunc("POST /api/sites/{id}/backups/restore", h.HandleRestoreBackup)
	mux.HandleFunc("DELETE /api/sites/{id}/backups/{filename}", h.HandleDeleteBackup)
}

func (h *BackupHandler) HandleCreateBackup(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	siteID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "ID không hợp lệ"}`, http.StatusBadRequest)
		return
	}

	info, err := h.engine.CreateBackup(siteID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"backup": info,
	})
}

func (h *BackupHandler) HandleListBackups(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	siteID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "ID không hợp lệ"}`, http.StatusBadRequest)
		return
	}

	backups, err := h.engine.ListBackups(siteID)
	if err != nil {
		http.Error(w, `{"error": "Không thể lấy danh sách sao lưu"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"backups": backups,
	})
}

func (h *BackupHandler) HandleRestoreBackup(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	siteID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "ID không hợp lệ"}`, http.StatusBadRequest)
		return
	}

	var body struct {
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Filename == "" {
		http.Error(w, `{"error": "Tên file không hợp lệ"}`, http.StatusBadRequest)
		return
	}

	if err := h.engine.RestoreBackup(siteID, body.Filename); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Đã khôi phục website thành công từ bản sao lưu",
	})
}

func (h *BackupHandler) HandleDeleteBackup(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	filename := r.PathValue("filename")
	siteID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "ID không hợp lệ"}`, http.StatusBadRequest)
		return
	}

	if err := h.engine.DeleteBackup(siteID, filename); err != nil {
		http.Error(w, `{"error": "Không thể xóa bản sao lưu"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Đã xóa bản sao lưu",
	})
}
