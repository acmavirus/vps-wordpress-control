package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"vps-wordpress-control/internal/config"
	"vps-wordpress-control/internal/models"
	"vps-wordpress-control/internal/scanner"
)

type SiteHandler struct {
	cfg       *config.Config
	db        *sql.DB
	discovery *scanner.DiscoveryEngine
}

func NewSiteHandler(cfg *config.Config, db *sql.DB) *SiteHandler {
	return &SiteHandler{
		cfg:       cfg,
		db:        db,
		discovery: scanner.NewDiscoveryEngine(cfg, db),
	}
}

func (h *SiteHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/discovery/scan", h.HandleTriggerScan)
	mux.HandleFunc("GET /api/sites", h.HandleListSites)
	mux.HandleFunc("GET /api/sites/{id}", h.HandleGetSiteDetail)
}

func (h *SiteHandler) HandleTriggerScan(w http.ResponseWriter, r *http.Request) {
	discovered, err := h.discovery.RunDiscovery()
	if err != nil {
		http.Error(w, `{"error": "Không thể hoàn thành quét website"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "success",
		"message":         "Quét và đồng bộ website hoàn tất",
		"sites_discovered": len(discovered),
		"sites":           discovered,
	})
}

func (h *SiteHandler) HandleListSites(w http.ResponseWriter, r *http.Request) {
	query := `
	SELECT 
		s.id, s.domain, s.root_path, s.php_version, s.wp_version, 
		s.owner_user, s.db_name, s.db_user, s.ssl_active, s.ssl_expires_at,
		s.fastcgi_cache, s.is_discovered, s.status, s.created_at, s.updated_at,
		(SELECT COUNT(id) FROM site_plugins WHERE site_id = s.id) as total_plugins,
		(SELECT COUNT(id) FROM site_plugins WHERE site_id = s.id AND has_update = 1) as pending_updates,
		(SELECT COUNT(id) FROM site_themes WHERE site_id = s.id) as total_themes,
		(SELECT COUNT(id) FROM site_themes WHERE site_id = s.id AND has_update = 1) as theme_updates
	FROM sites s
	ORDER BY s.id DESC;
	`

	rows, err := h.db.Query(query)
	if err != nil {
		http.Error(w, `{"error": "Lỗi truy vấn danh sách website"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var sites []models.Site
	for rows.Next() {
		var s models.Site
		var pendingUpdates, totalThemes, themeUpdates sql.NullInt64
		err := rows.Scan(
			&s.ID, &s.Domain, &s.RootPath, &s.PHPVersion, &s.WPVersion,
			&s.OwnerUser, &s.DBName, &s.DBUser, &s.SSLActive, &s.SSLExpiresAt,
			&s.FastCGICache, &s.IsDiscovered, &s.Status, &s.CreatedAt, &s.UpdatedAt,
			&s.TotalPlugins, &pendingUpdates, &totalThemes, &themeUpdates,
		)
		if err != nil {
			continue
		}
		if pendingUpdates.Valid {
			s.PendingUpdates = int(pendingUpdates.Int64)
		}
		if totalThemes.Valid {
			s.TotalThemes = int(totalThemes.Int64)
		}
		if themeUpdates.Valid {
			s.ThemeUpdates = int(themeUpdates.Int64)
		}
		sites = append(sites, s)
	}

	if sites == nil {
		sites = []models.Site{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   sites,
		"count":  len(sites),
	})
}

func (h *SiteHandler) HandleGetSiteDetail(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "ID không hợp lệ"}`, http.StatusBadRequest)
		return
	}

	var s models.Site
	query := `
	SELECT id, domain, root_path, php_version, wp_version, owner_user, db_name, db_user, ssl_active, status, created_at, updated_at
	FROM sites WHERE id = ?
	`
	err = h.db.QueryRow(query, id).Scan(
		&s.ID, &s.Domain, &s.RootPath, &s.PHPVersion, &s.WPVersion, &s.OwnerUser, &s.DBName, &s.DBUser, &s.SSLActive, &s.Status, &s.CreatedAt, &s.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		http.Error(w, `{"error": "Không tìm thấy website"}`, http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, `{"error": "Lỗi truy vấn"}`, http.StatusInternalServerError)
		return
	}

	// Lấy danh sách plugins
	pRows, err := h.db.Query("SELECT id, slug, name, version, update_version, status, has_update FROM site_plugins WHERE site_id = ?", id)
	var plugins []models.Plugin
	if err == nil {
		defer pRows.Close()
		for pRows.Next() {
			var p models.Plugin
			var upVer sql.NullString
			if err := pRows.Scan(&p.ID, &p.Slug, &p.Name, &p.Version, &upVer, &p.Status, &p.HasUpdate); err == nil {
				if upVer.Valid {
					p.UpdateVersion = upVer.String
				}
				p.SiteID = id
				plugins = append(plugins, p)
			}
		}
	}
	if plugins == nil {
		plugins = []models.Plugin{}
	}

	// Lấy danh sách themes
	tRows, err := h.db.Query("SELECT id, slug, name, version, update_version, status, has_update FROM site_themes WHERE site_id = ?", id)
	var themes []models.Theme
	if err == nil {
		defer tRows.Close()
		for tRows.Next() {
			var t models.Theme
			var upVer sql.NullString
			if err := tRows.Scan(&t.ID, &t.Slug, &t.Name, &t.Version, &upVer, &t.Status, &t.HasUpdate); err == nil {
				if upVer.Valid {
					t.UpdateVersion = upVer.String
				}
				t.SiteID = id
				themes = append(themes, t)
			}
		}
	}
	if themes == nil {
		themes = []models.Theme{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"site":    s,
		"plugins": plugins,
		"themes":  themes,
	})
}

