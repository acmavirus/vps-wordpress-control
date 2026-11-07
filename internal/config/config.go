package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	AppName              string
	AppEnv               string
	AppPort              string
	AppHost              string
	JWTSecret            string
	AdminDefaultUser     string
	AdminDefaultPassword string
	DataDir              string
	DBName               string
	NginxSitesDir        string
	NginxTmplDir         string
	WWWRootDir           string
	LogDir               string
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func LoadConfig() *Config {
	return &Config{
		AppName:       getEnv("APP_NAME", "VPS WordPress Control"),
		AppEnv:        getEnv("APP_ENV", "development"),
		AppPort:       getEnv("APP_PORT", "9090"),
		AppHost:       getEnv("APP_HOST", "0.0.0.0"),
		JWTSecret:            getEnv("JWT_SECRET", "default_insecure_jwt_secret"),
		AdminDefaultUser:     getEnv("ADMIN_DEFAULT_USER", "admin"),
		AdminDefaultPassword: getEnv("ADMIN_DEFAULT_PASSWORD", "admin_secret_123456"),
		DataDir:              getEnv("DATA_DIR", "./data"),
		DBName:        getEnv("DB_NAME", "wpcontrol.db"),
		NginxSitesDir: getEnv("NGINX_SITES_DIR", "/etc/nginx/sites-enabled"),
		NginxTmplDir:  getEnv("NGINX_TEMPLATES_DIR", "./templates"),
		WWWRootDir:    getEnv("WWW_ROOT_DIR", "/var/www"),
		LogDir:        getEnv("LOG_DIR", "./logs"),
	}
}

func (c *Config) GetDBPath() string {
	return filepath.Join(c.DataDir, c.DBName)
}

