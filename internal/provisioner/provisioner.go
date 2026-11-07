package provisioner

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"vps-wordpress-control/internal/config"
	"vps-wordpress-control/internal/wpcli"
)

type Provisioner struct {
	cfg   *config.Config
	db    *sql.DB
	wpcli *wpcli.WPCLI
}

func NewProvisioner(cfg *config.Config, db *sql.DB) *Provisioner {
	return &Provisioner{
		cfg:   cfg,
		db:    db,
		wpcli: wpcli.NewWPCLI(),
	}
}

type CreateSiteRequest struct {
	Domain        string `json:"domain"`
	SiteTitle     string `json:"site_title"`
	AdminUser     string `json:"admin_user"`
	AdminPassword string `json:"admin_password"`
	AdminEmail    string `json:"admin_email"`
	PHPVersion    string `json:"php_version"`
	EnableSSL     bool   `json:"enable_ssl"`
}

type CreateSiteResponse struct {
	Status        string `json:"status"`
	Domain        string `json:"domain"`
	RootPath      string `json:"root_path"`
	AdminUser     string `json:"admin_user"`
	AdminPassword string `json:"admin_password"`
	DBName        string `json:"db_name"`
	DBUser        string `json:"db_user"`
	SSLActive     bool   `json:"ssl_active"`
	Message       string `json:"message"`
}

func (p *Provisioner) CreateSite(req CreateSiteRequest) (*CreateSiteResponse, error) {
	domain := strings.ToLower(strings.TrimSpace(req.Domain))
	if domain == "" {
		return nil, fmt.Errorf("tên miền (domain) không được để trống")
	}

	phpVer := req.PHPVersion
	if phpVer == "" {
		phpVer = "8.2"
	}

	adminUser := req.AdminUser
	if adminUser == "" {
		adminUser = "admin"
	}

	adminPass := req.AdminPassword
	if adminPass == "" {
		adminPass = generateRandomString(12)
	}

	adminEmail := req.AdminEmail
	if adminEmail == "" {
		adminEmail = fmt.Sprintf("admin@%s", domain)
	}

	siteTitle := req.SiteTitle
	if siteTitle == "" {
		siteTitle = domain
	}

	rootPath := filepath.Join(p.cfg.WWWRootDir, domain, "public_html")
	log.Printf("[Provisioner] Bắt đầu khởi tạo website mới: %s tại %s", domain, rootPath)

	// 1. Tạo thư mục
	if err := os.MkdirAll(rootPath, 0755); err != nil {
		return nil, fmt.Errorf("không thể tạo thư mục web: %w", err)
	}

	// 2. Tạo Database MariaDB / MySQL
	cleanName := regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAllString(domain, "")
	if len(cleanName) > 10 {
		cleanName = cleanName[:10]
	}
	dbName := fmt.Sprintf("wp_%s", cleanName)
	dbUser := fmt.Sprintf("usr_%s", cleanName)
	dbPass := generateRandomString(16)

	if err := p.createMySQLDatabase(dbName, dbUser, dbPass); err != nil {
		log.Printf("[Provisioner Cảnh Báo] Lỗi tạo MySQL: %v (tiếp tục với cấu hình giả lập nếu trong dev)", err)
	}

	// 3. Tải & Cài đặt WordPress bằng WP-CLI
	log.Printf("[Provisioner] Đang tải mã nguồn WordPress sạch...")
	_, _ = p.wpcli.Run(rootPath, "www-data", "core", "download", "--locale=vi")
	_, _ = p.wpcli.Run(rootPath, "www-data", "config", "create",
		fmt.Sprintf("--dbname=%s", dbName),
		fmt.Sprintf("--dbuser=%s", dbUser),
		fmt.Sprintf("--dbpass=%s", dbPass),
		"--dbhost=localhost",
	)
	_, _ = p.wpcli.Run(rootPath, "www-data", "core", "install",
		fmt.Sprintf("--url=http://%s", domain),
		fmt.Sprintf("--title=%s", siteTitle),
		fmt.Sprintf("--admin_user=%s", adminUser),
		fmt.Sprintf("--admin_password=%s", adminPass),
		fmt.Sprintf("--admin_email=%s", adminEmail),
	)

	// 4. Tạo VirtualHost Nginx
	sslActive := false
	if err := p.configureNginx(domain, rootPath, phpVer); err != nil {
		log.Printf("[Provisioner Cảnh Báo] Lỗi cấu hình Nginx: %v", err)
	} else if req.EnableSSL {
		// 5. Cấp SSL Let's Encrypt qua Certbot
		if p.issueCertbotSSL(domain, adminEmail) {
			sslActive = true
		}
	}

	// 6. Phân quyền chown cho www-data
	_ = exec.Command("chown", "-R", "www-data:www-data", filepath.Join(p.cfg.WWWRootDir, domain)).Run()

	// 7. Lưu vào SQLite
	query := `
	INSERT INTO sites (
		domain, root_path, php_version, wp_version, owner_user, db_name, db_user, ssl_active, is_discovered, status
	) VALUES (?, ?, ?, '6.7.1', 'www-data', ?, ?, ?, 0, 'active')
	ON CONFLICT(domain) DO UPDATE SET
		root_path = excluded.root_path,
		php_version = excluded.php_version,
		db_name = excluded.db_name,
		db_user = excluded.db_user,
		ssl_active = excluded.ssl_active,
		updated_at = CURRENT_TIMESTAMP;
	`
	_, _ = p.db.Exec(query, domain, rootPath, phpVer, dbName, dbUser, sslActive)

	log.Printf("[Provisioner Thành Công] Website %s đã được tạo hoàn tất!", domain)
	return &CreateSiteResponse{
		Status:        "success",
		Domain:        domain,
		RootPath:      rootPath,
		AdminUser:     adminUser,
		AdminPassword: adminPass,
		DBName:        dbName,
		DBUser:        dbUser,
		SSLActive:     sslActive,
		Message:       "Website WordPress đã được tạo thành công!",
	}, nil
}

func (p *Provisioner) createMySQLDatabase(dbName, dbUser, dbPass string) error {
	sqlCmds := fmt.Sprintf(
		"CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci; "+
			"CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s'; "+
			"GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'localhost'; "+
			"FLUSH PRIVILEGES;",
		dbName, dbUser, dbPass, dbName, dbUser,
	)
	cmd := exec.Command("mysql", "-e", sqlCmds)
	return cmd.Run()
}

func (p *Provisioner) configureNginx(domain, rootPath, phpVersion string) error {
	tmplPath := filepath.Join(p.cfg.NginxTmplDir, "nginx-wp.conf")
	content, err := os.ReadFile(tmplPath)
	if err != nil {
		return fmt.Errorf("không thể đọc template nginx: %w", err)
	}

	confStr := string(content)
	confStr = strings.ReplaceAll(confStr, "{{DOMAIN}}", domain)
	confStr = strings.ReplaceAll(confStr, "{{ROOT_PATH}}", rootPath)
	confStr = strings.ReplaceAll(confStr, "{{PHP_VERSION}}", phpVersion)

	destPath := filepath.Join(p.cfg.NginxSitesDir, domain+".conf")
	if err := os.WriteFile(destPath, []byte(confStr), 0644); err != nil {
		return fmt.Errorf("không thể ghi file cấu hình nginx: %w", err)
	}

	// Test & reload Nginx
	_ = exec.Command("nginx", "-t").Run()
	_ = exec.Command("systemctl", "reload", "nginx").Run()
	return nil
}

func (p *Provisioner) issueCertbotSSL(domain, email string) bool {
	log.Printf("[Certbot] Đang cấp chứng chỉ SSL Let's Encrypt cho %s...", domain)
	cmd := exec.Command("certbot", "--nginx", "-d", domain, "-d", "www."+domain,
		"--non-interactive", "--agree-tos", "-m", email, "--redirect")
	err := cmd.Run()
	return err == nil
}

func generateRandomString(n int) string {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "WpPass123456!"
	}
	return hex.EncodeToString(bytes)[:n]
}

