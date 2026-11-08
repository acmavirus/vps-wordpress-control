# VPS WordPress Control 🚀

> **Lightweight Dedicated Control Panel** built specifically for managing multiple WordPress websites on VPS (Ubuntu / Debian).

![Go Version](https://img.shields.io/badge/Go-1.23-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/License-MIT-green.svg)
![Architecture](https://img.shields.io/badge/Architecture-amd64%20%7C%20arm64-blue)
![Languages](https://img.shields.io/badge/Languages-10%20Supported-purple)

---

## ✨ Key Features

1. **Auto-Discovery Engine:**
   - Automatically detects all existing WordPress sites by scanning Nginx configuration (`/etc/nginx/sites-enabled/`) and web directories.
   - Parses `wp-config.php`, inspects active/inactive plugins, themes, and PHP versions without altering existing site data or causing downtime.

2. **Multi-Language Support (i18n):**
   - Built-in support for **10 major languages**: 🇺🇸 English (*Default*), 🇻🇳 Vietnamese, 🇨🇳 Simplified Chinese, 🇯🇵 Japanese, 🇰🇷 Korean, 🇫🇷 French, 🇩🇪 German, 🇪🇸 Spanish, 🇷🇺 Russian, and 🇧🇷 Portuguese.
   - Instant language switching without page reloads, available on both the Login screen and Dashboard.

3. **Centralized WordPress Management:**
   - **Bulk Actions:** 1-click batch update for Plugins, Themes, and WordPress Core across selected or all sites.
   - **Fast PHP Switching:** Switch PHP versions (7.4 to 8.4) individually or in bulk; automatically updates Nginx FastCGI sockets and reloads services.
   - **SSL Automation:** Automated Let's Encrypt SSL certificate provisioning and Nginx HTTPS reconfiguration.
   - **Plugin Control:** Toggle plugins on/off directly from the Web UI to quickly resolve White Screen of Death (WSOD) errors.

4. **1-Click Magic Login (SSO):**
   - Generates secure, one-time admin login URLs allowing direct entry to `wp-admin` without remembering passwords.

5. **Multi-Layer Malware Scanner:**
   - **Checksum Verification:** Validates Core and Plugin integrity against official WordPress.org repositories (`wp core verify-checksums` & `wp plugin verify-checksums`).
   - **Uploads Sanitization:** Detects and flags dangerous executable scripts (`.php`, `.phtml`, `.phar`) inside `wp-content/uploads/`.
   - **Root Anomaly Detection:** Flags common webshells and suspicious files placed outside root directories (`wso.php`, `c99.php`, `shell.php`).
   - **Pattern Matching:** Deep regex inspection scanning for backdoors, obfuscation, and dangerous functions (`eval(base64_decode)`, `gzinflate`, `assert`, `RCE`).

6. **Fast Backup & Restore:**
   - 1-click complete archive of Database & `wp-content` into compressed `.tar.gz` files.
   - Instant rollback and restore directly from the web panel.

7. **1-Click Site Provisioning:**
   - Creates new WordPress installations from scratch: prepares web directory, sets up MariaDB database and user, configures secure Nginx VirtualHost, and generates SSL.

8. **Ultra-Lightweight & Secure:**
   - Written in **Go 1.23**, compiled into a single static binary (~15MB), RAM consumption < 30MB.
   - Smart automated port conflict resolution and firewall configuration (UFW & Firewalld).
   - Embedded responsive Tailwind CSS dark-mode dashboard — zero external web server dependencies.

---

## ⚡ Quick 1-Click Installation

Run the following command on your VPS (requires `root` privileges on Ubuntu 20.04/22.04/24.04 or Debian 11/12):

```bash
curl -sSL https://raw.githubusercontent.com/acmavirus/vps-wordpress-control/main/install.sh | sudo bash
```

The installer will automatically:
1. Detect your CPU architecture (`amd64` or `arm64`) and download the latest release.
2. Scan for available network ports (starts with `9090` and auto-increments if occupied).
3. Automatically configure and allow the selected port on UFW / Firewalld.
4. Set up the Systemd service and display your access credentials:
   - **Panel URL:** `http://<YOUR_VPS_IP>:<PORT>`
   - **Admin Username:** `admin`
   - **Admin Password:** *Randomly generated and stored securely in `/opt/wp-control/.env`*

---

## 🔄 1-Click Update

To update your panel to the latest version without losing data:

```bash
curl -sSL https://raw.githubusercontent.com/acmavirus/vps-wordpress-control/main/update.sh | sudo bash
```

*(Downloads the latest binary, replaces the executable, and restarts the service in under 3 seconds while preserving your configuration and database).*

---

## 🗑️ 1-Click Complete Uninstall

To completely and cleanly remove VPS WordPress Control from your server:

```bash
curl -sSL https://raw.githubusercontent.com/acmavirus/vps-wordpress-control/main/uninstall.sh | sudo bash
```

---

## 🛠 Service Management

```bash
# Check service status
sudo systemctl status wp-control

# Restart the panel
sudo systemctl restart wp-control

# View real-time logs
sudo journalctl -u wp-control -f
```

---

## 💻 Local Development (Docker)

```bash
# 1. Prepare environment configuration
cp .env.example .env

# 2. Start services with Docker Compose
docker compose up -d

# 3. Access Dashboard at
http://localhost:9090
```

---

## 📄 License
This project is licensed under the [MIT License](LICENSE).
