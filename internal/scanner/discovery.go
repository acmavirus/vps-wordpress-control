package scanner

import (
	"bufio"
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"vps-wordpress-control/internal/config"
	"vps-wordpress-control/internal/models"
	"vps-wordpress-control/internal/wpcli"
)

type DiscoveryEngine struct {
	cfg   *config.Config
	db    *sql.DB
	wpcli *wpcli.WPCLI
}

func NewDiscoveryEngine(cfg *config.Config, db *sql.DB) *DiscoveryEngine {
	return &DiscoveryEngine{cfg: cfg, db: db, wpcli: wpcli.NewWPCLI()}
}

type DiscoveredSite struct {
	Domain     string
	RootPath   string
	DBName     string
	DBUser     string
	WPVersion  string
	OwnerUser  string
	SSLActive  bool
	Plugins    []models.Plugin
}

// RunDiscovery tiến hành quét toàn bộ hệ thống file & Nginx để tìm các site WordPress
func (e *DiscoveryEngine) RunDiscovery() ([]DiscoveredSite, error) {
	log.Println("[Discovery] Bắt đầu quét hệ thống tìm kiếm website WordPress...")
	foundSites := make(map[string]*DiscoveredSite)

	// 1. Quét từ Nginx configuration
	e.scanNginxConfigs(foundSites)

	// 2. Quét dự phòng thư mục /var/www và /home
	e.scanCommonDirectories(foundSites)

	var results []DiscoveredSite
	for _, site := range foundSites {
		// Thẩm định chắc chắn đây là WordPress hợp lệ
		if e.validateAndEnrichWordPress(site) {
			results = append(results, *site)
			if err := e.saveDiscoveredSite(site); err != nil {
				log.Printf("[Discovery Lỗi] Không thể lưu site %s: %v", site.Domain, err)
			} else {
				log.Printf("[Discovery Thành Công] Đã nhận diện và lưu site: %s (%s)", site.Domain, site.RootPath)
			}
		}
	}

	log.Printf("[Discovery] Hoàn tất quét. Đã tìm thấy và đồng bộ %d website WordPress.", len(results))
	return results, nil
}

func (e *DiscoveryEngine) scanNginxConfigs(foundSites map[string]*DiscoveredSite) {
	nginxDir := e.cfg.NginxSitesDir
	if _, err := os.Stat(nginxDir); os.IsNotExist(err) {
		return
	}

	files, err := os.ReadDir(nginxDir)
	if err != nil {
		return
	}

	serverNameRegex := regexp.MustCompile(`(?i)^\s*server_name\s+([^;]+);`)
	rootRegex := regexp.MustCompile(`(?i)^\s*root\s+([^;]+);`)
	sslRegex := regexp.MustCompile(`(?i)^\s*ssl_certificate\s+`)

	for _, file := range files {
		path := filepath.Join(nginxDir, file.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		lines := strings.Split(string(content), "\n")
		var currentDomains []string
		var currentRoot string
		isSSL := false

		for _, line := range lines {
			if matches := serverNameRegex.FindStringSubmatch(line); len(matches) > 1 {
				domains := strings.Fields(matches[1])
				for _, d := range domains {
					d = strings.TrimSpace(d)
					if d != "_" && d != "localhost" && !strings.HasPrefix(d, "*") {
						currentDomains = append(currentDomains, d)
					}
				}
			}
			if matches := rootRegex.FindStringSubmatch(line); len(matches) > 1 {
				currentRoot = strings.TrimSpace(matches[1])
			}
			if sslRegex.MatchString(line) {
				isSSL = true
			}
		}

		if len(currentDomains) > 0 && currentRoot != "" {
			primaryDomain := currentDomains[0]
			if _, exists := foundSites[primaryDomain]; !exists {
				foundSites[primaryDomain] = &DiscoveredSite{
					Domain:    primaryDomain,
					RootPath:  currentRoot,
					SSLActive: isSSL,
				}
			}
		}
	}
}

func (e *DiscoveryEngine) scanCommonDirectories(foundSites map[string]*DiscoveredSite) {
	candidates := []string{e.cfg.WWWRootDir, "/home"}
	for _, baseDir := range candidates {
		if _, err := os.Stat(baseDir); os.IsNotExist(err) {
			continue
		}

		entries, err := os.ReadDir(baseDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			// Kiểm tra các trường hợp: /var/www/<domain>/public_html hoặc /var/www/<domain>/html hoặc /var/www/<domain>
			possibleRoots := []string{
				filepath.Join(baseDir, entry.Name(), "public_html"),
				filepath.Join(baseDir, entry.Name(), "html"),
				filepath.Join(baseDir, entry.Name(), "web"),
				filepath.Join(baseDir, entry.Name()),
			}

			for _, root := range possibleRoots {
				wpConfig := filepath.Join(root, "wp-config.php")
				if _, err := os.Stat(wpConfig); err == nil {
					domain := entry.Name()
					if _, exists := foundSites[domain]; !exists {
						foundSites[domain] = &DiscoveredSite{
							Domain:    domain,
							RootPath:  root,
							SSLActive: false,
						}
					}
					break
				}
			}
		}
	}
}

func (e *DiscoveryEngine) validateAndEnrichWordPress(site *DiscoveredSite) bool {
	wpConfigPath := filepath.Join(site.RootPath, "wp-config.php")
	// WordPress cũng hỗ trợ wp-config.php nằm ở thư mục cha 1 cấp
	if _, err := os.Stat(wpConfigPath); os.IsNotExist(err) {
		wpConfigPath = filepath.Join(site.RootPath, "..", "wp-config.php")
		if _, err := os.Stat(wpConfigPath); os.IsNotExist(err) {
			return false
		}
	}

	// Đọc cấu hình database từ wp-config.php
	content, err := os.ReadFile(wpConfigPath)
	if err == nil {
		dbNameRegex := regexp.MustCompile(`define\s*\(\s*['"]DB_NAME['"]\s*,\s*['"]([^'"]+)['"]\s*\)`)
		dbUserRegex := regexp.MustCompile(`define\s*\(\s*['"]DB_USER['"]\s*,\s*['"]([^'"]+)['"]\s*\)`)

		if matches := dbNameRegex.FindStringSubmatch(string(content)); len(matches) > 1 {
			site.DBName = matches[1]
		}
		if matches := dbUserRegex.FindStringSubmatch(string(content)); len(matches) > 1 {
			site.DBUser = matches[1]
		}
	}

	// Đọc phiên bản WordPress từ wp-includes/version.php
	versionPath := filepath.Join(site.RootPath, "wp-includes", "version.php")
	if vContent, err := os.ReadFile(versionPath); err == nil {
		verRegex := regexp.MustCompile(`\$wp_version\s*=\s*['"]([^'"]+)['"]`)
		if matches := verRegex.FindStringSubmatch(string(vContent)); len(matches) > 1 {
			site.WPVersion = matches[1]
		}
	}

	// Tự động nhận diện user sở hữu thư mục gốc/file wp-config.php trên hệ thống
	if fi, err := os.Stat(wpConfigPath); err == nil {
		site.OwnerUser = getOSFileOwner(fi)
	} else if fi, err := os.Stat(site.RootPath); err == nil {
		site.OwnerUser = getOSFileOwner(fi)
	}
	if site.OwnerUser == "" {
		site.OwnerUser = "www-data"
	}

	// Quét danh sách plugin từ thư mục wp-content/plugins
	pluginsDir := filepath.Join(site.RootPath, "wp-content", "plugins")
	site.Plugins = e.scanPluginsDirectory(pluginsDir)

	return true
}

func (e *DiscoveryEngine) scanPluginsDirectory(pluginsDir string) []models.Plugin {
	var plugins []models.Plugin
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return plugins
	}

	for _, entry := range entries {
		var mainFile string
		slug := entry.Name()

		if entry.IsDir() {
			// Thông thường file chính là <slug>/<slug>.php hoặc file PHP đầu tiên có header
			targetFile := filepath.Join(pluginsDir, slug, slug+".php")
			if _, err := os.Stat(targetFile); err == nil {
				mainFile = targetFile
			} else {
				// Tìm file php đầu tiên trong thư mục plugin
				subEntries, _ := os.ReadDir(filepath.Join(pluginsDir, slug))
				for _, sub := range subEntries {
					if strings.HasSuffix(sub.Name(), ".php") {
						mainFile = filepath.Join(pluginsDir, slug, sub.Name())
						break
					}
				}
			}
		} else if strings.HasSuffix(entry.Name(), ".php") && entry.Name() != "index.php" {
			mainFile = filepath.Join(pluginsDir, entry.Name())
			slug = strings.TrimSuffix(entry.Name(), ".php")
		}

		if mainFile != "" {
			name, ver := extractPluginHeader(mainFile)
			if name == "" {
				name = slug
			}
			plugins = append(plugins, models.Plugin{
				Slug:      slug,
				Name:      name,
				Version:   ver,
				Status:    "active", // Mặc định hiển thị
				HasUpdate: false,
			})
		}
	}
	return plugins
}

func extractPluginHeader(filePath string) (string, string) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	nameRegex := regexp.MustCompile(`(?i)^\s*\*\s*Plugin Name:\s*(.+)`)
	verRegex := regexp.MustCompile(`(?i)^\s*\*\s*Version:\s*(.+)`)

	var name, ver string
	lineCount := 0

	for scanner.Scan() && lineCount < 60 {
		line := scanner.Text()
		lineCount++

		if name == "" {
			if matches := nameRegex.FindStringSubmatch(line); len(matches) > 1 {
				name = strings.TrimSpace(matches[1])
			}
		}
		if ver == "" {
			if matches := verRegex.FindStringSubmatch(line); len(matches) > 1 {
				ver = strings.TrimSpace(matches[1])
			}
		}
		if name != "" && ver != "" {
			break
		}
	}
	return name, ver
}

func (e *DiscoveryEngine) saveDiscoveredSite(site *DiscoveredSite) error {
	query := `
	INSERT INTO sites (
		domain, root_path, owner_user, wp_version, db_name, db_user, ssl_active, is_discovered, status, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, 1, 'active', CURRENT_TIMESTAMP)
	ON CONFLICT(domain) DO UPDATE SET
		root_path = excluded.root_path,
		owner_user = excluded.owner_user,
		wp_version = excluded.wp_version,
		db_name = excluded.db_name,
		db_user = excluded.db_user,
		ssl_active = excluded.ssl_active,
		updated_at = CURRENT_TIMESTAMP
	RETURNING id;
	`
	var siteID int64
	err := e.db.QueryRow(query, site.Domain, site.RootPath, site.OwnerUser, site.WPVersion, site.DBName, site.DBUser, site.SSLActive).Scan(&siteID)
	if err != nil {
		return err
	}

	// Lấy danh sách slug plugin có update từ WP-CLI (tùy chọn — không làm hỏng nếu WP-CLI không có)
	pluginsWithUpdate := e.getPluginsWithUpdate(site.RootPath, site.OwnerUser)

	// Cập nhật danh sách plugins
	for _, p := range site.Plugins {
		hasUpdate := 0
		if pluginsWithUpdate[p.Slug] {
			hasUpdate = 1
		}
		pQuery := `
		INSERT INTO site_plugins (site_id, slug, name, version, status, has_update)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO NOTHING;
		`
		_, _ = e.db.Exec(pQuery, siteID, p.Slug, p.Name, p.Version, p.Status, hasUpdate)
	}

	// Cập nhật has_update cho các plugin đã tồn tại
	for slug, hasUpd := range pluginsWithUpdate {
		if hasUpd {
			_, _ = e.db.Exec("UPDATE site_plugins SET has_update = 1 WHERE site_id = ? AND slug = ?", siteID, slug)
		}
	}

	return nil
}

// getPluginsWithUpdate dùng WP-CLI để lấy danh sách plugin có bản cập nhật
// Trả về map[slug]true; bỏ qua hoàn toàn nếu WP-CLI không khả dụng
func (e *DiscoveryEngine) getPluginsWithUpdate(rootPath, ownerUser string) map[string]bool {
	result := make(map[string]bool)
	if ownerUser == "" {
		ownerUser = "www-data"
	}

	slugs, err := e.wpcli.GetPluginsWithUpdates(rootPath, ownerUser)
	if err != nil {
		// WP-CLI không có hoặc lỗi — silent skip, không làm ảnh hưởng discovery
		return result
	}

	for _, slug := range slugs {
		if slug != "" {
			result[slug] = true
		}
	}
	log.Printf("[Discovery] %s: Phát hiện %d plugin cần cập nhật.", rootPath, len(result))
	return result
}
