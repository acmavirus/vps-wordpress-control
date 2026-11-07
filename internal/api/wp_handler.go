package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"vps-wordpress-control/internal/config"
	"vps-wordpress-control/internal/provisioner"
	"vps-wordpress-control/internal/wpcli"
)

type WPHandler struct {
	cfg         *config.Config
	db          *sql.DB
	wpcli       *wpcli.WPCLI
	provisioner *provisioner.Provisioner
}

func NewWPHandler(cfg *config.Config, db *sql.DB) *WPHandler {
	return &WPHandler{
		cfg:         cfg,
		db:          db,
		wpcli:       wpcli.NewWPCLI(),
		provisioner: provisioner.NewProvisioner(cfg, db),
	}
}

func (h *WPHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/sites/create", h.HandleCreateSite)
	mux.HandleFunc("POST /api/sites/{id}/plugins/{slug}/update", h.HandleUpdatePlugin)
	mux.HandleFunc("POST /api/sites/{id}/plugins/update-all", h.HandleUpdateAllPlugins)
	mux.HandleFunc("POST /api/sites/{id}/plugins/{slug}/toggle", h.HandleTogglePlugin)
	mux.HandleFunc("GET /api/sites/{id}/magic-login", h.HandleMagicLogin)
}

func (h *WPHandler) HandleCreateSite(w http.ResponseWriter, r *http.Request) {
	var req provisioner.CreateSiteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Dữ liệu JSON không hợp lệ"}`, http.StatusBadRequest)
		return
	}

	resp, err := h.provisioner.CreateSite(req)
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
	json.NewEncoder(w).Encode(resp)
}

func (h *WPHandler) HandleUpdatePlugin(w http.ResponseWriter, r *http.Request) {
	siteID, slug, rootPath, ownerUser, err := h.getSiteInfo(r)
	if err != nil {
		http.Error(w, `{"error": "Không tìm thấy website"}`, http.StatusNotFound)
		return
	}

	if err := h.wpcli.UpdatePlugin(rootPath, ownerUser, slug); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	// Cập nhật trạng thái has_update = 0 trong database
	_, _ = h.db.Exec("UPDATE site_plugins SET has_update = 0 WHERE site_id = ? AND slug = ?", siteID, slug)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Cập nhật plugin thành công",
		"slug":    slug,
	})
}

func (h *WPHandler) HandleUpdateAllPlugins(w http.ResponseWriter, r *http.Request) {
	siteID, _, rootPath, ownerUser, err := h.getSiteInfo(r)
	if err != nil {
		http.Error(w, `{"error": "Không tìm thấy website"}`, http.StatusNotFound)
		return
	}

	if err := h.wpcli.UpdateAllPlugins(rootPath, ownerUser); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	// Đánh dấu tất cả plugin của site đã cập nhật
	_, _ = h.db.Exec("UPDATE site_plugins SET has_update = 0 WHERE site_id = ?", siteID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Đã cập nhật tất cả plugin của website thành công",
	})
}

func (h *WPHandler) HandleTogglePlugin(w http.ResponseWriter, r *http.Request) {
	siteID, slug, rootPath, ownerUser, err := h.getSiteInfo(r)
	if err != nil {
		http.Error(w, `{"error": "Không tìm thấy website"}`, http.StatusNotFound)
		return
	}

	var body struct {
		Active bool `json:"active"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	if err := h.wpcli.TogglePlugin(rootPath, ownerUser, slug, body.Active); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	newStatus := "inactive"
	if body.Active {
		newStatus = "active"
	}
	_, _ = h.db.Exec("UPDATE site_plugins SET status = ? WHERE site_id = ? AND slug = ?", newStatus, siteID, slug)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "success",
		"slug":       slug,
		"new_status": newStatus,
	})
}

func (h *WPHandler) HandleMagicLogin(w http.ResponseWriter, r *http.Request) {
	_, _, rootPath, ownerUser, err := h.getSiteInfo(r)
	if err != nil {
		http.Error(w, `{"error": "Không tìm thấy website"}`, http.StatusNotFound)
		return
	}

	url, err := h.wpcli.GenerateMagicLogin(rootPath, ownerUser, "admin")
	if err != nil {
		http.Error(w, `{"error": "Không thể sinh URL đăng nhập"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "success",
		"login_url": url,
	})
}

func (h *WPHandler) getSiteInfo(r *http.Request) (int64, string, string, string, error) {
	idStr := r.PathValue("id")
	slug := r.PathValue("slug")
	siteID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return 0, "", "", "", err
	}

	var rootPath, ownerUser string
	err = h.db.QueryRow("SELECT root_path, owner_user FROM sites WHERE id = ?", siteID).Scan(&rootPath, &ownerUser)
	if err != nil {
		return 0, "", "", "", err
	}
	return siteID, slug, rootPath, ownerUser, nil
}

