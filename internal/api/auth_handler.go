package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"vps-wordpress-control/internal/config"
)

type AuthHandler struct {
	cfg *config.Config
}

func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{cfg: cfg}
}

type TokenPayload struct {
	Username  string `json:"username"`
	ExpiresAt int64  `json:"exp"`
}

func (h *AuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/login", h.HandleLogin)
	mux.HandleFunc("GET /api/auth/me", h.HandleMe)
}

func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error": "Dữ liệu không hợp lệ"}`, http.StatusBadRequest)
		return
	}

	// Xác thực thông tin đăng nhập với cấu hình .env (hoặc admin cấu hình)
	expectedUser := h.cfg.AdminDefaultUser
	if expectedUser == "" {
		expectedUser = "admin"
	}
	expectedPass := h.cfg.AdminDefaultPassword

	if body.Username != expectedUser || body.Password != expectedPass {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"error":  "Tên đăng nhập hoặc mật khẩu không chính xác",
		})
		return
	}

	// Tạo Token HMAC-SHA256 tự ký (Hạn dùng 7 ngày)
	token, err := h.generateToken(body.Username, 7*24*time.Hour)
	if err != nil {
		http.Error(w, `{"error": "Lỗi sinh token"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "success",
		"token":    token,
		"username": body.Username,
	})
}

func (h *AuthHandler) HandleMe(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r)
	payload, ok := h.verifyToken(token)
	if !ok {
		http.Error(w, `{"error": "Chưa đăng nhập hoặc phiên làm việc hết hạn"}`, http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "success",
		"username": payload.Username,
	})
}

func (h *AuthHandler) generateToken(username string, duration time.Duration) (string, error) {
	payload := TokenPayload{
		Username:  username,
		ExpiresAt: time.Now().Add(duration).Unix(),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadBytes)

	// Ký chữ ký HMAC-SHA256 với secret key
	mac := hmac.New(sha256.New, []byte(h.cfg.JWTSecret))
	mac.Write([]byte(payloadB64))
	signatureHex := hex.EncodeToString(mac.Sum(nil))

	return payloadB64 + "." + signatureHex, nil
}

func (h *AuthHandler) verifyToken(tokenStr string) (*TokenPayload, bool) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 2 {
		return nil, false
	}

	payloadB64, signatureHex := parts[0], parts[1]

	// Kiểm tra chữ ký
	mac := hmac.New(sha256.New, []byte(h.cfg.JWTSecret))
	mac.Write([]byte(payloadB64))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signatureHex), []byte(expectedSig)) {
		return nil, false
	}

	// Giải mã payload
	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return nil, false
	}

	var payload TokenPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, false
	}

	// Kiểm tra thời hạn
	if time.Now().Unix() > payload.ExpiresAt {
		return nil, false
	}

	return &payload, true
}

func extractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	// Fallback đọc từ query param hoặc cookie nếu có
	if t := r.URL.Query().Get("token"); t != "" {
		return t
	}
	return ""
}

// Middleware xác thực bảo vệ toàn bộ các endpoint /api/ (ngoại trừ /api/health và /api/auth/login)
// Đồng thời kiểm tra IP Whitelist nếu biến ALLOWED_IPS được cấu hình
func (h *AuthHandler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Kiểm tra IP Whitelist nếu cấu hình ALLOWED_IPS không rỗng
		if h.cfg.AllowedIPs != "" && !h.isIPAllowed(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"status": "error", "error": "Truy cập bị từ chối: IP không nằm trong danh sách cho phép"}`))
			return
		}

		path := r.URL.Path

		// Bỏ qua xác thực cho các route công khai
		if path == "/" || path == "/index.html" || path == "/favicon.ico" || path == "/api/health" || path == "/api/auth/login" {
			next.ServeHTTP(w, r)
			return
		}

		// Với tất cả các API khác, bắt buộc phải có token hợp lệ
		if strings.HasPrefix(path, "/api/") {
			token := extractToken(r)
			if _, ok := h.verifyToken(token); !ok {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"status": "error", "error": "Yêu cầu đăng nhập để truy cập"}`))
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func (h *AuthHandler) isIPAllowed(r *http.Request) bool {
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP != "" {
		parts := strings.Split(clientIP, ",")
		clientIP = strings.TrimSpace(parts[0])
	}
	if clientIP == "" {
		clientIP = r.Header.Get("X-Real-IP")
	}
	if clientIP == "" {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err == nil {
			clientIP = host
		} else {
			clientIP = r.RemoteAddr
		}
	}

	allowedList := strings.Split(h.cfg.AllowedIPs, ",")
	for _, allowed := range allowedList {
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			continue
		}
		// Hỗ trợ kiểm tra CIDR (VD: 192.168.1.0/24) hoặc exact IP
		if strings.Contains(allowed, "/") {
			_, ipNet, err := net.ParseCIDR(allowed)
			if err == nil && ipNet.Contains(net.ParseIP(clientIP)) {
				return true
			}
		} else if allowed == clientIP {
			return true
		}
	}
	return false
}

