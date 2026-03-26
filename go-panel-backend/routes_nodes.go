package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (s *Server) handleNodeCreate(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if _, err := s.requireAdmin(r); err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	var req NodeDTO
	if err := s.decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(-1, "请求参数错误"))
		return
	}
	if err := validatePortRange(req.PortSta, req.PortEnd); err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, err.Error()))
		return
	}
	now := time.Now().UnixMilli()
	_, err := s.db.Exec(`
		INSERT INTO node (name, secret, ip, server_ip, port_sta, port_end, version, http, tls, socks, created_time, updated_time, status)
		VALUES (?, ?, ?, ?, ?, ?, '', 0, 0, 0, ?, ?, 0)`,
		strings.TrimSpace(req.Name),
		randomToken(32),
		strings.TrimSpace(req.IP),
		strings.TrimSpace(req.ServerIP),
		req.PortSta,
		req.PortEnd,
		now,
		now,
	)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "节点创建失败"))
		return
	}
	writeJSON(w, http.StatusOK, ok("节点创建成功"))
}

func (s *Server) handleNodeList(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if _, err := s.requireAdmin(r); err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	nodes, err := s.listNodes()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(-2, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, ok(nodes))
}

func (s *Server) handleNodeUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if _, err := s.requireAdmin(r); err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	var req NodeUpdateDTO
	if err := s.decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(-1, "请求参数错误"))
		return
	}
	node, err := s.getNodeByID(req.ID)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "节点不存在"))
		return
	}
	if err := validatePortRange(req.PortSta, req.PortEnd); err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, err.Error()))
		return
	}
	httpValue := node.HTTP
	tlsValue := node.TLS
	socksValue := node.Socks
	if req.HTTP != nil {
		httpValue = *req.HTTP
	}
	if req.TLS != nil {
		tlsValue = *req.TLS
	}
	if req.Socks != nil {
		socksValue = *req.Socks
	}

	if node.Status == 1 && (httpValue != node.HTTP || tlsValue != node.TLS || socksValue != node.Socks) {
		result := s.sendNodeCommand(node.ID, "SetProtocol", map[string]interface{}{
			"http":  httpValue,
			"tls":   tlsValue,
			"socks": socksValue,
		})
		if result.Msg != "OK" {
			writeJSON(w, http.StatusOK, errResp(-1, result.Msg))
			return
		}
	}

	_, err = s.db.Exec(`
		UPDATE node
		SET name = ?, ip = ?, server_ip = ?, port_sta = ?, port_end = ?, http = ?, tls = ?, socks = ?, updated_time = ?
		WHERE id = ?`,
		strings.TrimSpace(req.Name),
		strings.TrimSpace(req.IP),
		strings.TrimSpace(req.ServerIP),
		req.PortSta,
		req.PortEnd,
		httpValue,
		tlsValue,
		socksValue,
		time.Now().UnixMilli(),
		req.ID,
	)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "节点更新失败"))
		return
	}
	_, _ = s.db.Exec(`UPDATE tunnel SET in_ip = ? WHERE in_node_id = ?`, req.IP, req.ID)
	_, _ = s.db.Exec(`UPDATE tunnel SET out_ip = ? WHERE out_node_id = ?`, req.ServerIP, req.ID)
	writeJSON(w, http.StatusOK, ok("节点更新成功"))
}

func (s *Server) handleNodeDelete(w http.ResponseWriter, r *http.Request) {
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
	if _, err := s.getNodeByID(id); err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "节点不存在"))
		return
	}
	var inUseCount int
	_ = s.db.QueryRow(`SELECT COUNT(1) FROM tunnel WHERE in_node_id = ?`, id).Scan(&inUseCount)
	if inUseCount > 0 {
		writeJSON(w, http.StatusOK, errResp(-1, fmt.Sprintf("该节点还有%d个隧道作为入口节点在使用，请先删除相关隧道", inUseCount)))
		return
	}
	var outUseCount int
	_ = s.db.QueryRow(`SELECT COUNT(1) FROM tunnel WHERE out_node_id = ?`, id).Scan(&outUseCount)
	if outUseCount > 0 {
		writeJSON(w, http.StatusOK, errResp(-1, fmt.Sprintf("该节点还有%d个隧道作为出口节点在使用，请先删除相关隧道", outUseCount)))
		return
	}
	if _, err := s.db.Exec(`DELETE FROM node WHERE id = ?`, id); err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "节点删除失败"))
		return
	}
	writeJSON(w, http.StatusOK, ok("节点删除成功"))
}

func (s *Server) handleNodeInstallCommand(w http.ResponseWriter, r *http.Request) {
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
	node, err := s.getNodeByID(id)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "节点不存在"))
		return
	}
	cfg, err := s.getConfigByName("ip")
	if err != nil || strings.TrimSpace(cfg.Value) == "" {
		writeJSON(w, http.StatusOK, errResp(-1, "请先前往网站配置中设置ip"))
		return
	}
	command := fmt.Sprintf(
		"curl -fsSL https://raw.githubusercontent.com/bqlpfy/flux-panel/main/install.sh -o ./install.sh && chmod 700 ./install.sh && ./install.sh -a %s -s %s",
		processServerAddress(cfg.Value),
		node.Secret,
	)
	writeJSON(w, http.StatusOK, ok(command))
}

func (s *Server) handleNodeCheckStatus(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if _, err := s.requireAdmin(r); err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	var params map[string]interface{}
	_ = s.decodeJSON(r, &params)
	if params != nil && params["nodeId"] != nil {
		node, err := s.getNodeByID(mapInt64(params["nodeId"]))
		if err != nil {
			writeJSON(w, http.StatusOK, errResp(-1, "节点不存在"))
			return
		}
		writeJSON(w, http.StatusOK, ok(map[string]interface{}{
			"nodeId": node.ID,
			"status": node.Status,
			"online": node.Status == 1,
		}))
		return
	}
	nodes, err := s.listNodes()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(-2, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, ok(nodes))
}

func (s *Server) handleNodeWSTicket(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	claims, err := s.requireAdmin(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	userID, _ := strconv.ParseInt(claims.Sub, 10, 64)
	ticket := s.ticketStore.New(userID, claims.RoleID)
	writeJSON(w, http.StatusOK, ok(map[string]string{
		"ticket": ticket.Value,
	}))
}

func validatePortRange(start, end int) error {
	if start < 1 || start > 65535 || end < 1 || end > 65535 {
		return errors.New("端口必须在1-65535范围内")
	}
	if end < start {
		return errors.New("结束端口不能小于起始端口")
	}
	return nil
}

func processServerAddress(serverAddr string) string {
	serverAddr = strings.TrimSpace(serverAddr)
	if serverAddr == "" || strings.HasPrefix(serverAddr, "[") {
		return serverAddr
	}
	lastColon := strings.LastIndex(serverAddr, ":")
	if lastColon == -1 {
		if strings.Count(serverAddr, ":") >= 2 {
			return "[" + serverAddr + "]"
		}
		return serverAddr
	}
	host := serverAddr[:lastColon]
	if strings.Count(host, ":") >= 2 {
		return "[" + host + "]" + serverAddr[lastColon:]
	}
	return serverAddr
}
