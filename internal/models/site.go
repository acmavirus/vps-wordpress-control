package models

import "time"

// Site đại diện cho một website WordPress trong hệ thống
type Site struct {
	ID             int64      `json:"id"`
	Domain         string     `json:"domain"`
	RootPath       string     `json:"root_path"`
	PHPVersion     string     `json:"php_version"`
	WPVersion      string     `json:"wp_version"`
	OwnerUser      string     `json:"owner_user"`
	DBName         string     `json:"db_name"`
	DBUser         string     `json:"db_user"`
	SSLActive      bool       `json:"ssl_active"`
	SSLExpiresAt   *time.Time `json:"ssl_expires_at,omitempty"`
	FastCGICache   bool       `json:"fastcgi_cache"`
	IsDiscovered   bool       `json:"is_discovered"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	TotalPlugins   int        `json:"total_plugins,omitempty"`
	PendingUpdates int        `json:"pending_updates,omitempty"`
	TotalThemes    int        `json:"total_themes,omitempty"`
	ThemeUpdates   int        `json:"theme_updates,omitempty"`
}

// Theme đại diện cho một theme của website WordPress
type Theme struct {
	ID            int64  `json:"id"`
	SiteID        int64  `json:"site_id"`
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	UpdateVersion string `json:"update_version,omitempty"`
	Status        string `json:"status"` // active, inactive
	HasUpdate     bool   `json:"has_update"`
}

// Plugin đại diện cho một plugin của website WordPress
type Plugin struct {
	ID            int64  `json:"id"`
	SiteID        int64  `json:"site_id"`
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	UpdateVersion string `json:"update_version,omitempty"`
	Status        string `json:"status"` // active, inactive
	HasUpdate     bool   `json:"has_update"`
}

// ScanResult đại diện cho một cảnh báo mã độc hoặc tính toàn vẹn
type ScanResult struct {
	ID               int64     `json:"id"`
	SiteID           int64     `json:"site_id"`
	ScanType         string    `json:"scan_type"` // checksum, upload_php, malware_pattern
	FilePath         string    `json:"file_path"`
	IssueDescription string    `json:"issue_description"`
	Severity         string    `json:"severity"` // info, warning, critical
	DetectedAt       time.Time `json:"detected_at"`
}

// Job đại diện cho một tác vụ chạy ngầm
type Job struct {
	ID        int64     `json:"id"`
	SiteID    *int64    `json:"site_id,omitempty"`
	Action    string    `json:"action"` // scan_all, update_all_plugins, create_site
	Status    string    `json:"status"` // pending, running, completed, failed
	Progress  int       `json:"progress"`
	LogOutput string    `json:"log_output"`
	CreatedAt time.Time `json:"created_at"`
}

