package api

import (
	"encoding/json"
	"net/http"
	"novel-api/config"
	"strings"
)

// ConfigResponse 配置查询响应
type ConfigResponse struct {
	Success bool   `json:"success"`
	Data    ConfigData `json:"data"`
	Message string `json:"message,omitempty"`
}

type ConfigData struct {
	NovelAIBaseURL string `json:"novel_ai_base_url"`
	NovelAIKey     string `json:"novel_ai_key"`
}

// ConfigUpdateRequest 配置更新请求
type ConfigUpdateRequest struct {
	NovelAIBaseURL string `json:"novel_ai_base_url"`
	NovelAIKey     string `json:"novel_ai_key"`
}

// GetConfig 获取配置API
func GetConfig(w http.ResponseWriter, r *http.Request, cfg *config.Config) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 验证token
	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	if !isValidToken(token) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "未授权访问",
		})
		return
	}

	response := ConfigResponse{
		Success: true,
		Data: ConfigData{
			NovelAIBaseURL: cfg.NovelAI.BaseURL,
			NovelAIKey:     cfg.NovelAI.Key,
		},
	}
	json.NewEncoder(w).Encode(response)
}

// UpdateConfig 更新配置API (内存更新)
func UpdateConfig(w http.ResponseWriter, r *http.Request, cfg *config.Config) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 验证token
	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	if !isValidToken(token) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "未授权访问",
		})
		return
	}

	var req ConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "无效的请求格式",
		})
		return
	}

	// 更新内存中的配置
	cfg.NovelAI.BaseURL = req.NovelAIBaseURL
	cfg.NovelAI.Key = req.NovelAIKey

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "配置更新成功",
	})
}