package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (s *Server) handleSpeedLimitCreate(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if _, err := s.requireAdmin(r); err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	var req SpeedLimitDTO
	if err := s.decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(-1, "请求参数错误"))
		return
	}
	tunnel, err := s.getTunnelByID(req.TunnelID)
	if err != nil || tunnel.Name != req.TunnelName {
		writeJSON(w, http.StatusOK, errResp(-1, "指定的隧道不存在"))
		return
	}
	now := time.Now().UnixMilli()
	result, err := s.db.Exec(`
		INSERT INTO speed_limit (name, speed, tunnel_id, tunnel_name, created_time, updated_time, status)
		VALUES (?, ?, ?, ?, ?, ?, 1)`,
		strings.TrimSpace(req.Name), req.Speed, req.TunnelID, req.TunnelName, now, now,
	)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "限速规则创建失败"))
		return
	}
	id, _ := result.LastInsertId()
	speedMB := formatMbps(req.Speed)
	gost := s.sendNodeCommand(tunnel.InNodeID, "AddLimiters", limiterPayload(id, speedMB))
	if gost.Msg != "OK" {
		_, _ = s.db.Exec(`UPDATE speed_limit SET status = 0 WHERE id = ?`, id)
		_, _ = s.db.Exec(`DELETE FROM speed_limit WHERE id = ?`, id)
		writeJSON(w, http.StatusOK, errResp(-1, gost.Msg))
		return
	}
	writeJSON(w, http.StatusOK, okNoData())
}

func (s *Server) handleSpeedLimitList(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if _, err := s.requireAdmin(r); err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	items, err := s.listSpeedLimits()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(-2, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, ok(items))
}

func (s *Server) handleSpeedLimitUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if _, err := s.requireAdmin(r); err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	var req SpeedLimitUpdateDTO
	if err := s.decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(-1, "请求参数错误"))
		return
	}
	item, err := s.getSpeedLimitByID(req.ID)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "限速规则不存在"))
		return
	}
	tunnel, err := s.getTunnelByID(req.TunnelID)
	if err != nil || tunnel.Name != req.TunnelName {
		writeJSON(w, http.StatusOK, errResp(-1, "隧道名称与隧道ID不匹配"))
		return
	}
	_, err = s.db.Exec(`
		UPDATE speed_limit
		SET name = ?, speed = ?, tunnel_id = ?, tunnel_name = ?, updated_time = ?
		WHERE id = ?`,
		req.Name, req.Speed, req.TunnelID, req.TunnelName, time.Now().UnixMilli(), req.ID,
	)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "限速规则更新失败"))
		return
	}
	speedMB := formatMbps(req.Speed)
	payload := map[string]interface{}{
		"limiter": fmt.Sprintf("%d", item.ID),
		"data":    limiterPayload(item.ID, speedMB),
	}
	gost := s.sendNodeCommand(tunnel.InNodeID, "UpdateLimiters", payload)
	if strings.Contains(strings.ToLower(gost.Msg), "not found") {
		gost = s.sendNodeCommand(tunnel.InNodeID, "AddLimiters", limiterPayload(item.ID, speedMB))
	}
	if gost.Msg != "OK" {
		writeJSON(w, http.StatusOK, errResp(-1, gost.Msg))
		return
	}
	writeJSON(w, http.StatusOK, ok("限速规则更新成功"))
}

func (s *Server) handleSpeedLimitDelete(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if _, err := s.requireAdmin(r); err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	var params map[string]interface{}
	if err := s.decodeJSON(r, &params); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(-1, "请求参数错误"))
		return
	}
	id := mapInt64(params["id"])
	item, err := s.getSpeedLimitByID(id)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "限速规则不存在"))
		return
	}
	var inUse int
	_ = s.db.QueryRow(`SELECT COUNT(1) FROM user_tunnel WHERE speed_id = ?`, id).Scan(&inUse)
	if inUse > 0 {
		writeJSON(w, http.StatusOK, errResp(-1, "该限速规则还有用户在使用 请先取消分配"))
		return
	}
	if tunnel, err := s.getTunnelByID(item.TunnelID); err == nil {
		_ = s.sendNodeCommand(tunnel.InNodeID, "DeleteLimiters", map[string]interface{}{
			"limiter": fmt.Sprintf("%d", id),
		})
	}
	if _, err := s.db.Exec(`DELETE FROM speed_limit WHERE id = ?`, id); err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "限速规则删除失败"))
		return
	}
	writeJSON(w, http.StatusOK, ok("限速规则删除成功"))
}

func (s *Server) handleSpeedLimitTunnels(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if _, err := s.requireAdmin(r); err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	tunnels, err := s.listTunnels()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(-2, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, ok(tunnels))
}

func formatMbps(speed int) string {
	return strconv.FormatFloat(float64(speed)/8.0, 'f', 1, 64)
}
