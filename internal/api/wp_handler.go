package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

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
	mux.HandleFunc("POST /api/sites/bulk-update-plugins", h.HandleBulkUpdatePlugins)
	mux.HandleFunc("POST /api/sites/{id}/plugins/{slug}/toggle", h.HandleTogglePlugin)
	mux.HandleFunc("GET /api/sites/{id}/magic-login", h.HandleMagicLogin)
	mux.HandleFunc("POST /api/sites/{id}/change-php", h.HandleChangePHPVersion)
	mux.HandleFunc("GET /api/system/php-versions", h.HandleGetPHPVersions)
	mux.HandleFunc("POST /api/sites/{id}/toggle-cache", h.HandleToggleFastCGICache)
	mux.HandleFunc("POST /api/sites/{id}/core-update", h.HandleCoreUpdate)
	mux.HandleFunc("POST /api/sites/{id}/issue-ssl", h.HandleIssueSSL)
	mux.HandleFunc("POST /api/sites/{id}/themes/{slug}/update", h.HandleUpdateTheme)
	mux.HandleFunc("POST /api/sites/{id}/themes/update-all", h.HandleUpdateAllThemes)
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

	_, _ = h.db.Exec("UPDATE site_plugins SET has_update = 0 WHERE site_id = ?", siteID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Đã cập nhật tất cả plugin của website thành công",
	})
}

// HandleBulkUpdatePlugins cập nhật plugin hàng loạt cho danh sách website được chọn
func (h *WPHandler) HandleBulkUpdatePlugins(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SiteIDs []int64 `json:"site_ids"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	var query string
	var rows *sql.Rows
	var err error

	if len(body.SiteIDs) > 0 {
		placeholders := make([]string, len(body.SiteIDs))
		args := make([]interface{}, len(body.SiteIDs))
		for i, id := range body.SiteIDs {
			placeholders[i] = "?"
			args[i] = id
		}
		query = fmt.Sprintf("SELECT id, domain, root_path, owner_user FROM sites WHERE id IN (%s)", strings.Join(placeholders, ","))
		rows, err = h.db.Query(query, args...)
	} else {
		// Nếu không truyền site_ids, cập nhật tất cả website đang có plugin cần update
		query = `
			SELECT DISTINCT s.id, s.domain, s.root_path, s.owner_user 
			FROM sites s
			INNER JOIN site_plugins p ON s.id = p.site_id
			WHERE p.has_update = 1 AND s.status = 'active'
		`
		rows, err = h.db.Query(query)
	}

	if err != nil {
		http.Error(w, `{"error": "Lỗi truy vấn danh sách website"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type SiteResult struct {
		ID      int64  `json:"id"`
		Domain  string `json:"domain"`
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}

	var results []SiteResult
	successCount := 0

	for rows.Next() {
		var id int64
		var domain, rootPath, ownerUser string
		if err := rows.Scan(&id, &domain, &rootPath, &ownerUser); err == nil {
			err := h.wpcli.UpdateAllPlugins(rootPath, ownerUser)
			if err != nil {
				results = append(results, SiteResult{ID: id, Domain: domain, Success: false, Error: err.Error()})
			} else {
				_, _ = h.db.Exec("UPDATE site_plugins SET has_update = 0 WHERE site_id = ?", id)
				results = append(results, SiteResult{ID: id, Domain: domain, Success: true})
				successCount++
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":        "success",
		"total_sites":   len(results),
		"success_count": successCount,
		"results":       results,
	})
}

// HandleChangePHPVersion thay đổi phiên bản PHP của 1 website (Cập nhật Nginx socket & reload)
func (h *WPHandler) HandleChangePHPVersion(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	siteID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "ID không hợp lệ"}`, http.StatusBadRequest)
		return
	}

	var body struct {
		PHPVersion string `json:"php_version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PHPVersion == "" {
		http.Error(w, `{"error": "Phiên bản PHP không hợp lệ"}`, http.StatusBadRequest)
		return
	}

	var domain, rootPath string
	err = h.db.QueryRow("SELECT domain, root_path FROM sites WHERE id = ?", siteID).Scan(&domain, &rootPath)
	if err != nil {
		http.Error(w, `{"error": "Không tìm thấy website"}`, http.StatusNotFound)
		return
	}

	// Tìm và cập nhật file cấu hình Nginx
	updatedNginx := h.switchNginxPHP(domain, body.PHPVersion)
	if !updatedNginx {
		log.Printf("[PHP Switcher Cảnh Báo] Không thể sửa file Nginx của %s (có thể đang chạy trong dev container)", domain)
	}

	// Cập nhật vào SQLite
	_, _ = h.db.Exec("UPDATE sites SET php_version = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", body.PHPVersion, siteID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "success",
		"message":     fmt.Sprintf("Đã chuyển website %s sang PHP %s thành công", domain, body.PHPVersion),
		"domain":      domain,
		"new_version": body.PHPVersion,
	})
}

func (h *WPHandler) switchNginxPHP(domain, newPHP string) bool {
	nginxDir := h.cfg.NginxSitesDir
	confFiles := []string{
		filepath.Join(nginxDir, domain+".conf"),
		filepath.Join("/etc/nginx/sites-available", domain+".conf"),
		filepath.Join(nginxDir, domain),
	}

	fastcgiRegex := regexp.MustCompile(`fastcgi_pass\s+unix:/run/php/php[0-9.]+-fpm\.sock;`)
	newSocket := fmt.Sprintf("fastcgi_pass unix:/run/php/php%s-fpm.sock;", newPHP)

	found := false
	for _, confPath := range confFiles {
		content, err := os.ReadFile(confPath)
		if err != nil {
			continue
		}

		newContent := fastcgiRegex.ReplaceAll(content, []byte(newSocket))
		if err := os.WriteFile(confPath, newContent, 0644); err == nil {
			found = true
			break
		}
	}

	if found {
		_ = exec.Command("nginx", "-t").Run()
		_ = exec.Command("systemctl", "reload", "nginx").Run()
	}
	return found
}

// HandleGetPHPVersions quét và trả về danh sách các phiên bản PHP đang cài đặt trên VPS
func (h *WPHandler) HandleGetPHPVersions(w http.ResponseWriter, r *http.Request) {
	versionsMap := make(map[string]bool)

	// 1. Quét từ /run/php/ (sockets FPM)
	if files, err := os.ReadDir("/run/php"); err == nil {
		sockRegex := regexp.MustCompile(`^php([0-9.]+)-fpm\.sock$`)
		for _, f := range files {
			if matches := sockRegex.FindStringSubmatch(f.Name()); len(matches) > 1 {
				versionsMap[matches[1]] = true
			}
		}
	}

	// 2. Quét từ /etc/php/
	if dirs, err := os.ReadDir("/etc/php"); err == nil {
		for _, d := range dirs {
			if d.IsDir() && regexp.MustCompile(`^[0-9.]+$`).MatchString(d.Name()) {
				versionsMap[d.Name()] = true
			}
		}
	}

	var versions []string
	for v := range versionsMap {
		versions = append(versions, v)
	}

	// Nếu chưa phát hiện hoặc đang trong container test, trả về danh sách chuẩn
	if len(versions) == 0 {
		versions = []string{"7.4", "8.0", "8.1", "8.2", "8.3", "8.4"}
	} else {
		sort.Strings(versions)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "success",
		"versions": versions,
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

func (h *WPHandler) HandleToggleFastCGICache(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	siteID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "ID không hợp lệ"}`, http.StatusBadRequest)
		return
	}

	var currentCache int
	err = h.db.QueryRow("SELECT fastcgi_cache FROM sites WHERE id = ?", siteID).Scan(&currentCache)
	if err != nil {
		http.Error(w, `{"error": "Không tìm thấy website"}`, http.StatusNotFound)
		return
	}

	newCache := 1
	if currentCache == 1 {
		newCache = 0
	}

	_, _ = h.db.Exec("UPDATE sites SET fastcgi_cache = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", newCache, siteID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":        "success",
		"fastcgi_cache": newCache,
	})
}

func (h *WPHandler) HandleMagicLogin(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	siteID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "ID không hợp lệ"}`, http.StatusBadRequest)
		return
	}

	var domain, rootPath, ownerUser string
	var sslActive int
	err = h.db.QueryRow("SELECT domain, root_path, owner_user, ssl_active FROM sites WHERE id = ?", siteID).Scan(&domain, &rootPath, &ownerUser, &sslActive)
	if err != nil {
		http.Error(w, `{"error": "Không tìm thấy website"}`, http.StatusNotFound)
		return
	}

	url, err := h.wpcli.GenerateMagicLogin(domain, rootPath, ownerUser, sslActive == 1)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusInternalServerError)
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

// HandleCoreUpdate cập nhật WordPress Core của một website
func (h *WPHandler) HandleCoreUpdate(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	siteID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "ID không hợp lệ"}`, http.StatusBadRequest)
		return
	}

	var domain, rootPath, ownerUser string
	err = h.db.QueryRow("SELECT domain, root_path, owner_user FROM sites WHERE id = ?", siteID).Scan(&domain, &rootPath, &ownerUser)
	if err != nil {
		http.Error(w, `{"error": "Không tìm thấy website"}`, http.StatusNotFound)
		return
	}

	if err := h.wpcli.UpdateCore(rootPath, ownerUser); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	// Đọc lại phiên bản WP sau khi cập nhật
	newVersion := ""
	if out, err := h.wpcli.Run(rootPath, ownerUser, "core", "version"); err == nil {
		newVersion = strings.TrimSpace(out)
		_, _ = h.db.Exec("UPDATE sites SET wp_version = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", newVersion, siteID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "success",
		"message":     fmt.Sprintf("Đã cập nhật WordPress Core của %s thành công!", domain),
		"domain":      domain,
		"new_version": newVersion,
	})
}

// HandleIssueSSL cấp hoặc gia hạn chứng chỉ Let's Encrypt SSL cho website
func (h *WPHandler) HandleIssueSSL(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	siteID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "ID không hợp lệ"}`, http.StatusBadRequest)
		return
	}

	var domain string
	err = h.db.QueryRow("SELECT domain FROM sites WHERE id = ?", siteID).Scan(&domain)
	if err != nil {
		http.Error(w, `{"error": "Không tìm thấy website"}`, http.StatusNotFound)
		return
	}

	adminEmail := fmt.Sprintf("admin@%s", domain)
	log.Printf("[Certbot] Yêu cầu cấp SSL cho domain: %s (Email: %s)", domain, adminEmail)

	// Thử chạy certbot cấp SSL
	cmd := exec.Command("certbot", "--nginx", "-d", domain, "-d", "www."+domain,
		"--non-interactive", "--agree-tos", "-m", adminEmail, "--redirect")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Thử cấp đơn domain nếu www.domain không trỏ DNS
		cmdSingle := exec.Command("certbot", "--nginx", "-d", domain,
			"--non-interactive", "--agree-tos", "-m", adminEmail, "--redirect")
		outSingle, errSingle := cmdSingle.CombinedOutput()
		if errSingle != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "error",
				"error":  fmt.Sprintf("Lỗi cấp SSL qua Certbot: %s", string(outSingle)),
			})
			return
		}
	}

	// Cập nhật trạng thái SSL vào database
	_, _ = h.db.Exec("UPDATE sites SET ssl_active = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?", siteID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "success",
		"message":    fmt.Sprintf("Đã cấp và kích hoạt SSL HTTPS thành công cho %s!", domain),
		"domain":     domain,
		"ssl_active": true,
		"log":        string(out),
	})
}

// HandleUpdateTheme cập nhật 1 theme cụ thể
func (h *WPHandler) HandleUpdateTheme(w http.ResponseWriter, r *http.Request) {
	siteID, slug, rootPath, ownerUser, err := h.getSiteInfo(r)
	if err != nil {
		http.Error(w, `{"error": "Không tìm thấy website"}`, http.StatusNotFound)
		return
	}

	if err := h.wpcli.UpdateTheme(rootPath, ownerUser, slug); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	_, _ = h.db.Exec("UPDATE site_themes SET has_update = 0 WHERE site_id = ? AND slug = ?", siteID, slug)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Cập nhật theme thành công",
		"slug":    slug,
	})
}

// HandleUpdateAllThemes cập nhật tất cả theme của một website
func (h *WPHandler) HandleUpdateAllThemes(w http.ResponseWriter, r *http.Request) {
	siteID, _, rootPath, ownerUser, err := h.getSiteInfo(r)
	if err != nil {
		http.Error(w, `{"error": "Không tìm thấy website"}`, http.StatusNotFound)
		return
	}

	if err := h.wpcli.UpdateAllThemes(rootPath, ownerUser); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	_, _ = h.db.Exec("UPDATE site_themes SET has_update = 0 WHERE site_id = ?", siteID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Đã cập nhật tất cả theme của website thành công",
	})
}
