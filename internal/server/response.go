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
