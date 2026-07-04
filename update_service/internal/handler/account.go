package handler

import (
	"encoding/json"
	"net/http"

	"shadow-worker/update_service/internal/middleware"
	"shadow-worker/update_service/internal/service"
)

// AccountHandler 处理账号相关请求（改密码）。
type AccountHandler struct {
	authSvc *service.AuthService
}

// NewAccountHandler 创建 AccountHandler。
func NewAccountHandler(authSvc *service.AuthService) *AccountHandler {
	return &AccountHandler{authSvc: authSvc}
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// ChangePassword 处理改密码。username 从 JWT context 取（不信任请求体）。
func (h *AccountHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	username := middleware.UsernameFromContext(r.Context())
	if username == "" {
		http.Error(w, `{"error":"unauthenticated"}`, http.StatusUnauthorized)
		return
	}

	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.OldPassword == "" || req.NewPassword == "" {
		http.Error(w, `{"error":"old_password and new_password are required"}`, http.StatusBadRequest)
		return
	}

	if err := h.authSvc.ChangePassword(username, req.OldPassword, req.NewPassword); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}
