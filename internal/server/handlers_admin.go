package server

import (
	"net/http"

	"vps/internal/proxyman"
)

// handleListUsers 对应 HandlerService GetInboundUsers，列举某 inbound 的用户。
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	tag, ok := requireParam(w, r, "tag")
	if !ok {
		return
	}
	ctx, cancel := s.withTimeout(r)
	defer cancel()
	data, err := s.pm.ListUsers(ctx, tag)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// handleCountUsers 对应 HandlerService GetInboundUsersCount，返回某 inbound 的用户数。
func (s *Server) handleCountUsers(w http.ResponseWriter, r *http.Request) {
	tag, ok := requireParam(w, r, "tag")
	if !ok {
		return
	}
	ctx, cancel := s.withTimeout(r)
	defer cancel()
	data, err := s.pm.CountUsers(ctx, tag)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// handleAddUser 对应 HandlerService AlterInbound + AddUserOperation，新增一个 VLESS 用户。
func (s *Server) handleAddUser(w http.ResponseWriter, r *http.Request) {
	var req proxyman.AddUserRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Tag == "" || req.Email == "" || req.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tag、email、id 均为必填"})
		return
	}
	ctx, cancel := s.withTimeout(r)
	defer cancel()
	if err := s.pm.AddVLESSUser(ctx, req.Tag, req.Email, req.ID, req.Flow, req.Level); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "email": req.Email})
}

// handleRemoveUser 对应 HandlerService AlterInbound + RemoveUserOperation，按 email 删除用户。
func (s *Server) handleRemoveUser(w http.ResponseWriter, r *http.Request) {
	tag, ok := requireParam(w, r, "tag")
	if !ok {
		return
	}
	email, ok := requireParam(w, r, "email")
	if !ok {
		return
	}
	ctx, cancel := s.withTimeout(r)
	defer cancel()
	if err := s.pm.RemoveUser(ctx, tag, email); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "email": email})
}

// handleListRules 对应 RoutingService ListRule，列举当前路由规则。
func (s *Server) handleListRules(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.withTimeout(r)
	defer cancel()
	data, err := s.routing.ListRules(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}
