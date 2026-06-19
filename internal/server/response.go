package server

import (
	"encoding/json"
	"net/http"
)

// writeJSON 以美化后的 JSON 写出响应。
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

// writeError 返回 500 + 错误信息。
func writeError(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
}

// queryBool 解析查询参数为布尔值，仅 "true"/"1" 视为 true。
func queryBool(r *http.Request, key string) bool {
	switch r.URL.Query().Get(key) {
	case "true", "1":
		return true
	default:
		return false
	}
}

// requireParam 读取必填查询参数；缺失时写出 400 并返回 ok=false。
func requireParam(w http.ResponseWriter, r *http.Request, key string) (string, bool) {
	v := r.URL.Query().Get(key)
	if v == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少必填查询参数: " + key})
		return "", false
	}
	return v, true
}

// decodeJSON 解析请求 body 到 dst；失败时写出 400 并返回 ok=false。
func decodeJSON(w http.ResponseWriter, r *http.Request, dst interface{}) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效的 JSON body: " + err.Error()})
		return false
	}
	return true
}

// requireToken 包裹一个 handler，校验 Authorization: Bearer <adminToken>。
// adminToken 未配置或不匹配时返回 403，handler 不会被调用。
func (s *Server) requireToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.adminToken == "" {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "写操作未启用：未配置 ADMIN_TOKEN"})
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+s.adminToken {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "无效或缺失的鉴权 token"})
			return
		}
		next(w, r)
	}
}
