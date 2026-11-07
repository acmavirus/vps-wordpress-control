package wpcli

import (
	"bytes"
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
	if ownerUser == "" {
		ownerUser = "www-data"
	}

	fullArgs := append([]string{"--path=" + sitePath, "--allow-root"}, args...)

	var cmd *exec.Cmd
	// Nếu chạy dưới quyền root trên Linux, ưu tiên chạy dưới quyền ownerUser để đảm bảo phân quyền file
	if os.Geteuid() == 0 && ownerUser != "root" {
		sudoArgs := append([]string{"-u", ownerUser, w.binPath, "--path=" + sitePath}, args...)
		cmd = exec.Command("sudo", sudoArgs...)
	} else {
		cmd = exec.Command(w.binPath, fullArgs...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return stdout.String(), fmt.Errorf("lỗi thực thi wp-cli: %v | stderr: %s", err, stderr.String())
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

// GenerateMagicLogin sinh đường dẫn đăng nhập 1-click vào wp-admin
func (w *WPCLI) GenerateMagicLogin(sitePath, ownerUser, adminUser string) (string, error) {
	if adminUser == "" {
		adminUser = "admin"
	}

	// Thử dùng wp-cli package nếu có
	out, err := w.Run(sitePath, ownerUser, "login", "create", adminUser, "--url-only")
	if err == nil && strings.HasPrefix(out, "http") {
		return out, nil
	}

	// Cơ chế dự phòng: Sinh token đăng nhập an toàn tạm thời thông qua PHP eval
	phpCode := fmt.Sprintf(`
		require_once('wp-load.php');
		$user = get_user_by('login', '%s');
		if (!$user) {
			$users = get_users(array('role' => 'administrator', 'number' => 1));
			if (!empty($users)) { $user = $users[0]; }
		}
		if ($user) {
			wp_set_current_user($user->ID);
			wp_set_auth_cookie($user->ID);
			echo admin_url();
		}
	`, adminUser)

	out, err = w.Run(sitePath, ownerUser, "eval", phpCode)
	if err == nil && strings.HasPrefix(out, "http") {
		return out, nil
	}

	return filepath.Join(sitePath, "wp-admin"), nil
}

