package wpcli

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type WPCLI struct {
	binPath string
}

func NewWPCLI() *WPCLI {
	bin := "wp"
	if p, err := exec.LookPath("wp"); err == nil {
		bin = p
	} else if _, err := os.Stat("/usr/local/bin/wp"); err == nil {
		bin = "/usr/local/bin/wp"
	}
	return &WPCLI{binPath: bin}
}

// Run thực thi lệnh wp-cli với đường dẫn site và phân quyền an toàn
func (w *WPCLI) Run(sitePath string, ownerUser string, args ...string) (string, error) {
	var cmd *exec.Cmd
	// Nếu đang chạy dưới quyền root trên Linux và có user sở hữu khác root:
	// Chạy dưới quyền ownerUser (không dùng --allow-root)
	if os.Geteuid() == 0 && ownerUser != "" && ownerUser != "root" {
		sudoArgs := append([]string{"-u", ownerUser, w.binPath, "--path=" + sitePath}, args...)
		cmd = exec.Command("sudo", sudoArgs...)
	} else {
		// Ngược lại chạy trực tiếp với --allow-root
		fullArgs := append([]string{"--path=" + sitePath, "--allow-root"}, args...)
		cmd = exec.Command(w.binPath, fullArgs...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// FALLBACK: Nếu lệnh chạy dưới quyền user thất bại do Permission Denied (ví dụ user không đủ quyền ghi file cũ của root),
		// tự động thử lại trực tiếp với quyền root (--allow-root) để hoàn tất tác vụ (như update-core, plugin update).
		if os.Geteuid() == 0 && ownerUser != "" && ownerUser != "root" {
			fallbackArgs := append([]string{"--path=" + sitePath, "--allow-root"}, args...)
			fallbackCmd := exec.Command(w.binPath, fallbackArgs...)
			var fbStdout, fbStderr bytes.Buffer
			fallbackCmd.Stdout = &fbStdout
			fallbackCmd.Stderr = &fbStderr
			if fbErr := fallbackCmd.Run(); fbErr == nil {
				// Sửa lại quyền toàn bộ thư mục về đúng user sở hữu để tránh lỗi quyền cho các lần sau
				_ = exec.Command("chown", "-R", ownerUser+":"+ownerUser, sitePath).Run()
				return strings.TrimSpace(fbStdout.String()), nil
			}
		}
		return stdout.String(), fmt.Errorf("lỗi thực thi wp-cli: %v | stderr: %s", err, stderr.String())
	}

	// Đảm bảo sau khi cập nhật thành công, phân quyền vẫn thuộc ownerUser nếu có
	if os.Geteuid() == 0 && ownerUser != "" && ownerUser != "root" {
		_ = exec.Command("chown", "-R", ownerUser+":"+ownerUser, sitePath).Run()
	}

	return strings.TrimSpace(stdout.String()), nil
}

// UpdatePlugin cập nhật 1 plugin cụ thể
func (w *WPCLI) UpdatePlugin(sitePath, ownerUser, slug string) error {
	_, err := w.Run(sitePath, ownerUser, "plugin", "update", slug)
	return err
}

// UpdateAllPlugins cập nhật tất cả plugin của website
func (w *WPCLI) UpdateAllPlugins(sitePath, ownerUser string) error {
	_, err := w.Run(sitePath, ownerUser, "plugin", "update", "--all")
	return err
}

// UpdateTheme cập nhật 1 theme cụ thể
func (w *WPCLI) UpdateTheme(sitePath, ownerUser, slug string) error {
	_, err := w.Run(sitePath, ownerUser, "theme", "update", slug)
	return err
}

// UpdateAllThemes cập nhật tất cả theme của website
func (w *WPCLI) UpdateAllThemes(sitePath, ownerUser string) error {
	_, err := w.Run(sitePath, ownerUser, "theme", "update", "--all")
	return err
}

// GetThemesWithUpdates trả về danh sách slug theme có bản cập nhật mới
func (w *WPCLI) GetThemesWithUpdates(sitePath, ownerUser string) ([]string, error) {
	out, err := w.Run(sitePath, ownerUser, "theme", "list",
		"--update=available", "--fields=name", "--format=csv", "--skip-plugins", "--skip-themes")
	if err != nil {
		return nil, err
	}

	var slugs []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && line != "name" && line != "slug" {
			slugs = append(slugs, line)
		}
	}
	return slugs, nil
}

// TogglePlugin bật hoặc tắt 1 plugin (hữu ích để fix lỗi trắng trang WSOD)
func (w *WPCLI) TogglePlugin(sitePath, ownerUser, slug string, activate bool) error {
	action := "deactivate"
	if activate {
		action = "activate"
	}
	_, err := w.Run(sitePath, ownerUser, "plugin", action, slug)
	return err
}

// UpdateCore cập nhật phiên bản WordPress Core
func (w *WPCLI) UpdateCore(sitePath, ownerUser string) error {
	_, err := w.Run(sitePath, ownerUser, "core", "update")
	return err
}

// RunCombined thực thi wp-cli và trả về stdout + stderr kể cả khi lệnh thất bại
// Hữu ích để parse output của các lệnh checksum
func (w *WPCLI) RunCombined(sitePath string, ownerUser string, args ...string) string {
	if ownerUser == "" {
		ownerUser = "www-data"
	}

	fullArgs := append([]string{"--path=" + sitePath, "--allow-root"}, args...)

	var cmd *exec.Cmd
	if os.Geteuid() == 0 && ownerUser != "root" {
		sudoArgs := append([]string{"-u", ownerUser, w.binPath, "--path=" + sitePath}, args...)
		cmd = exec.Command("sudo", sudoArgs...)
	} else {
		cmd = exec.Command(w.binPath, fullArgs...)
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	_ = cmd.Run() // Bỏ qua lỗi exit code, lấy toàn bộ output để parse
	return strings.TrimSpace(out.String())
}

// GetPluginsWithUpdates trả về danh sách slug plugin có bản cập nhật mới
func (w *WPCLI) GetPluginsWithUpdates(sitePath, ownerUser string) ([]string, error) {
	out, err := w.Run(sitePath, ownerUser, "plugin", "list",
		"--update=available", "--fields=slug", "--format=csv", "--skip-plugins", "--skip-themes")
	if err != nil {
		return nil, err
	}

	var slugs []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && line != "slug" {
			slugs = append(slugs, line)
		}
	}
	return slugs, nil
}

// GenerateMagicLogin sinh đường dẫn đăng nhập 1-click vào wp-admin an toàn bằng One-Time Token
func (w *WPCLI) GenerateMagicLogin(domain, sitePath, ownerUser string, isSSL bool) (string, error) {
	tokenBytes := make([]byte, 24)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("không thể sinh token")
	}
	token := hex.EncodeToString(tokenBytes)

	// Tạo file đăng nhập tạm thời an toàn trong thư mục web: .wp_magic_<token>.php
	scriptName := fmt.Sprintf(".wp_magic_%s.php", token[:12])
	scriptPath := filepath.Join(sitePath, scriptName)

	phpContent := fmt.Sprintf(`<?php
// VPS WordPress Control - One-Time Auto Login
if (!isset($_GET['token']) || $_GET['token'] !== '%s') {
    http_response_code(403);
    exit('Truy cập bị từ chối');
}

// Xóa file ngay lập tức sau lần truy cập đầu tiên để đảm bảo chỉ dùng 1 lần
@unlink(__FILE__);

define('WP_USE_THEMES', false);
require_once __DIR__ . '/wp-load.php';

$users = get_users(['role' => 'administrator', 'number' => 1]);
if (empty($users)) {
    exit('Không tìm thấy tài khoản quản trị viên.');
}

$admin = $users[0];
wp_clear_auth_cookie();
wp_set_current_user($admin->ID);
wp_set_auth_cookie($admin->ID, true, is_ssl());

wp_safe_redirect(admin_url());
exit;
`, token)

	if err := os.WriteFile(scriptPath, []byte(phpContent), 0644); err != nil {
		return "", fmt.Errorf("không thể tạo file đăng nhập tạm: %w", err)
	}

	// Đảm bảo owner là user sở hữu website để web server (PHP-FPM) có thể đọc và xóa file sau khi login
	if os.Geteuid() == 0 && ownerUser != "" && ownerUser != "root" {
		_ = exec.Command("chown", ownerUser+":"+ownerUser, scriptPath).Run()
	}

	scheme := "http"
	if isSSL {
		scheme = "https"
	}

	loginURL := fmt.Sprintf("%s://%s/%s?token=%s", scheme, domain, scriptName, token)
	return loginURL, nil
}

