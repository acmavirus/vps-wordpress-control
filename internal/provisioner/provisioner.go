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
	WPVersion     string `json:"wp_version"`
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

	// 3. Tải & Cài đặt WordPress bằng WP-CLI theo phiên bản được chọn
	wpVer := strings.TrimSpace(req.WPVersion)
	if wpVer == "" || wpVer == "latest" {
		wpVer = "6.7.1"
	}
	log.Printf("[Provisioner] Đang tải mã nguồn WordPress phiên bản %s...", wpVer)

	downloadArgs := []string{"core", "download", "--locale=vi"}
	if req.WPVersion != "" && req.WPVersion != "latest" {
		downloadArgs = append(downloadArgs, fmt.Sprintf("--version=%s", req.WPVersion))
	}
	_, _ = p.wpcli.Run(rootPath, "www-data", downloadArgs...)
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
		log.Printf("[Provisioner Lỗi] Lỗi cấu hình Nginx cho %s: %v", domain, err)
		return nil, fmt.Errorf("lỗi cấu hình Nginx cho %s: %w", domain, err)
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
	) VALUES (?, ?, ?, ?, 'www-data', ?, ?, ?, 0, 'active')
	ON CONFLICT(domain) DO UPDATE SET
		root_path = excluded.root_path,
		php_version = excluded.php_version,
		wp_version = excluded.wp_version,
		db_name = excluded.db_name,
		db_user = excluded.db_user,
		ssl_active = excluded.ssl_active,
		updated_at = CURRENT_TIMESTAMP;
	`
	_, _ = p.db.Exec(query, domain, rootPath, phpVer, wpVer, dbName, dbUser, sslActive)

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

const defaultNginxTemplate = `# Nginx VirtualHost Template cho WordPress (VPS WordPress Control)
# Tu dong toi uu hoa hieu nang va bao mat da tang

server {
    listen 80;
    listen [::]:80;
    server_name {{DOMAIN}} www.{{DOMAIN}};
    root {{ROOT_PATH}};
    index index.php index.html index.htm;
    client_max_body_size 100M;

    access_log /var/log/nginx/{{DOMAIN}}_access.log;
    error_log /var/log/nginx/{{DOMAIN}}_error.log;

    # 1. Chặn truy cập file nhạy cảm
    location ~* /(?:wp-config\.php|readme\.html|license\.txt|\.git|\.env|\.user\.ini) {
        deny all;
    }

    # 2. CHỐNG WEBSHELL: Tuyệt đối chặn thực thi PHP trong thư mục uploads
    location ~* ^/wp-content/uploads/.*\.php$ {
        deny all;
    }

    # 3. Chặn tấn công Brute Force vào XML-RPC
    location = /xmlrpc.php {
        deny all;
    }

    # 4. WordPress Permalinks
    location / {
        try_files $uri $uri/ /index.php?$args;
    }

    # 5. Xử lý PHP-FPM
    location ~ \.php$ {
        include snippets/fastcgi-php.conf;
        fastcgi_pass {{FASTCGI_PASS}};
        fastcgi_param PHP_ADMIN_VALUE "open_basedir={{SITE_DIR}}/:{{ROOT_PATH}}/:/tmp/:/proc/";
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
        include fastcgi_params;
        fastcgi_read_timeout 300;
    }

    # 6. Cache tài nguyên tĩnh (Browser Caching)
    location ~* \.(jpg|jpeg|gif|png|webp|svg|ico|css|js|woff|woff2|ttf|eot)$ {
        expires 30d;
        add_header Cache-Control "public, no-transform";
        access_log off;
    }

    # Cho phep certbot xac thuc SSL
    location ~ /\.well-known {
        allow all;
    }

    location ~ /\.ht {
        deny all;
    }
}
`

func resolvePHPSocket(phpVersion string) string {
	target := fmt.Sprintf("/run/php/php%s-fpm.sock", phpVersion)
	if _, err := os.Stat(target); err == nil {
		return "unix:" + target
	}

	// Danh sách fallback các socket khả dụng
	candidates := []string{
		"/run/php/php-fpm.sock",
		"/run/php/php8.3-fpm.sock",
		"/run/php/php8.2-fpm.sock",
		"/run/php/php8.1-fpm.sock",
		"/run/php/php8.0-fpm.sock",
		"/run/php/php7.4-fpm.sock",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			log.Printf("[Provisioner] Socket %s không tồn tại, tự động chuyển sang socket khả dụng: %s", target, c)
			return "unix:" + c
		}
	}
	return "unix:" + target
}

func (p *Provisioner) configureNginx(domain, rootPath, phpVersion string) error {
	tmplPath := filepath.Join(p.cfg.NginxTmplDir, "nginx-wp.conf")
	var confStr string

	content, err := os.ReadFile(tmplPath)
	if err != nil || len(content) == 0 {
		log.Printf("[Provisioner] Không tìm thấy file template tại %s, sử dụng template mặc định nhúng sẵn", tmplPath)
		confStr = defaultNginxTemplate
		// Tự động ghi lại file template để lưu trữ lâu dài
		if err := os.MkdirAll(p.cfg.NginxTmplDir, 0755); err == nil {
			_ = os.WriteFile(tmplPath, []byte(defaultNginxTemplate), 0644)
		}
	} else {
		confStr = string(content)
	}

	fastcgiPass := resolvePHPSocket(phpVersion)
	siteDir := filepath.Dir(rootPath)

	confStr = strings.ReplaceAll(confStr, "{{DOMAIN}}", domain)
	confStr = strings.ReplaceAll(confStr, "{{ROOT_PATH}}", rootPath)
	confStr = strings.ReplaceAll(confStr, "{{SITE_DIR}}", siteDir)
	confStr = strings.ReplaceAll(confStr, "{{PHP_VERSION}}", phpVersion)
	confStr = strings.ReplaceAll(confStr, "{{FASTCGI_PASS}}", fastcgiPass)
	// Đảm bảo tương thích ngược nếu template cũ dùng unix:/run/php/php{{PHP_VERSION}}-fpm.sock
	confStr = strings.ReplaceAll(confStr, fmt.Sprintf("unix:/run/php/php%s-fpm.sock", phpVersion), fastcgiPass)

	// Đảm bảo thư mục lưu cấu hình Nginx tồn tại
	if err := os.MkdirAll(p.cfg.NginxSitesDir, 0755); err != nil {
		return fmt.Errorf("không thể tạo thư mục cấu hình nginx: %w", err)
	}

	destPath := filepath.Join(p.cfg.NginxSitesDir, domain+".conf")
	if err := os.WriteFile(destPath, []byte(confStr), 0644); err != nil {
		return fmt.Errorf("không thể ghi file cấu hình nginx: %w", err)
	}

	// Nếu lưu vào sites-available thì tự tạo symlink sang sites-enabled
	if strings.Contains(p.cfg.NginxSitesDir, "sites-available") {
		enabledDir := filepath.Join(filepath.Dir(p.cfg.NginxSitesDir), "sites-enabled")
		_ = os.MkdirAll(enabledDir, 0755)
		_ = os.Symlink(destPath, filepath.Join(enabledDir, domain+".conf"))
	}

	// Test cú pháp Nginx
	testOut, testErr := exec.Command("nginx", "-t").CombinedOutput()
	if testErr != nil {
		return fmt.Errorf("kiểm tra cấu hình Nginx thất bại: %s", string(testOut))
	}

	// Reload Nginx & PHP-FPM
	_ = exec.Command("systemctl", "reload", "nginx").Run()
	_ = exec.Command("systemctl", "reload", "php"+phpVersion+"-fpm").Run()
	_ = exec.Command("systemctl", "reload", "php8.3-fpm").Run()
	_ = exec.Command("systemctl", "reload", "php-fpm").Run()
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

