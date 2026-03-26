package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (s *Server) handleTunnelCreate(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if _, err := s.requireAdmin(r); err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	var req TunnelDTO
	if err := s.decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(-1, "请求参数错误"))
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSON(w, http.StatusOK, errResp(-1, "隧道名称不能为空"))
		return
	}
	if existing, err := s.db.Query(`SELECT id FROM tunnel WHERE name = ?`, req.Name); err == nil {
		existing.Close()
	}
	var dup int
	_ = s.db.QueryRow(`SELECT COUNT(1) FROM tunnel WHERE name = ?`, req.Name).Scan(&dup)
	if dup > 0 {
		writeJSON(w, http.StatusOK, errResp(-1, "隧道名称已存在"))
		return
	}
	inNode, err := s.getNodeByID(req.InNodeID)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "入口节点不存在"))
		return
	}
	if inNode.Status != 1 {
		writeJSON(w, http.StatusOK, errResp(-1, "入口节点当前离线，请确保节点正常运行"))
		return
	}
	outNodeID := req.InNodeID
	outIP := inNode.ServerIP
	protocol := strings.TrimSpace(req.Protocol)
	if protocol == "" {
		protocol = "tls"
	}
	if req.Type == 2 {
		if req.OutNodeID == nil {
			writeJSON(w, http.StatusOK, errResp(-1, "出口节点不能为空"))
			return
		}
		if *req.OutNodeID == req.InNodeID {
			writeJSON(w, http.StatusOK, errResp(-1, "隧道转发模式下，入口和出口不能是同一个节点"))
			return
		}
		outNode, err := s.getNodeByID(*req.OutNodeID)
		if err != nil {
			writeJSON(w, http.StatusOK, errResp(-1, "出口节点不存在"))
			return
		}
		if outNode.Status != 1 {
			writeJSON(w, http.StatusOK, errResp(-1, "出口节点当前离线，请确保节点正常运行"))
			return
		}
		outNodeID = outNode.ID
		outIP = outNode.ServerIP
		if protocol == "" {
			writeJSON(w, http.StatusOK, errResp(-1, "协议类型必填"))
			return
		}
	}
	trafficRatio := 1.0
	if req.TrafficRatio != nil && *req.TrafficRatio > 0 {
		trafficRatio = *req.TrafficRatio
	}
	tcpListen := strings.TrimSpace(req.TCPListenAddr)
	udpListen := strings.TrimSpace(req.UDPListenAddr)
	if tcpListen == "" {
		tcpListen = "[::]"
	}
	if udpListen == "" {
		udpListen = "[::]"
	}
	now := time.Now().UnixMilli()
	_, err = s.db.Exec(`
		INSERT INTO tunnel (name, traffic_ratio, in_node_id, in_ip, out_node_id, out_ip, type, protocol, flow, tcp_listen_addr, udp_listen_addr, interface_name, created_time, updated_time, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)`,
		req.Name, trafficRatio, inNode.ID, inNode.IP, outNodeID, outIP, req.Type, protocol, req.Flow, tcpListen, udpListen, strings.TrimSpace(req.InterfaceName), now, now,
	)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "隧道创建失败"))
		return
	}
	writeJSON(w, http.StatusOK, ok("隧道创建成功"))
}

func (s *Server) handleTunnelList(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if _, err := s.requireAdmin(r); err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	items, err := s.listTunnels()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(-2, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, ok(items))
}

func (s *Server) handleTunnelGet(w http.ResponseWriter, r *http.Request) {
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
	item, err := s.getTunnelByID(mapInt64(params["id"]))
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "隧道不存在"))
		return
	}
	writeJSON(w, http.StatusOK, ok(item))
}

func (s *Server) handleTunnelUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if _, err := s.requireAdmin(r); err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	var req TunnelUpdateDTO
	if err := s.decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(-1, "请求参数错误"))
		return
	}
	tunnel, err := s.getTunnelByID(req.ID)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "隧道不存在"))
		return
	}
	var dup int
	_ = s.db.QueryRow(`SELECT COUNT(1) FROM tunnel WHERE name = ? AND id <> ?`, req.Name, req.ID).Scan(&dup)
	if dup > 0 {
		writeJSON(w, http.StatusOK, errResp(-1, "隧道名称已存在"))
		return
	}
	interfaceName := tunnel.InterfaceName
	if req.InterfaceName != nil {
		interfaceName = strings.TrimSpace(*req.InterfaceName)
	}
	trafficRatio := tunnel.TrafficRatio
	if req.TrafficRatio != nil && *req.TrafficRatio > 0 {
		trafficRatio = *req.TrafficRatio
	}
	networkChanged := tunnel.TCPListenAddr != req.TCPListenAddr ||
		tunnel.UDPListenAddr != req.UDPListenAddr ||
		tunnel.Protocol != req.Protocol ||
		tunnel.InterfaceName != interfaceName

	_, err = s.db.Exec(`
		UPDATE tunnel
		SET name = ?, flow = ?, traffic_ratio = ?, protocol = ?, tcp_listen_addr = ?, udp_listen_addr = ?, interface_name = ?, updated_time = ?
		WHERE id = ?`,
		req.Name, req.Flow, trafficRatio, req.Protocol, req.TCPListenAddr, req.UDPListenAddr, interfaceName, time.Now().UnixMilli(), req.ID,
	)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "隧道更新失败"))
		return
	}
	if networkChanged {
		forwards, _ := s.listForwardsByTunnel(req.ID)
		for _, forward := range forwards {
			_ = s.syncForwardRuntime(forward.ID)
		}
	}
	writeJSON(w, http.StatusOK, ok("隧道更新成功"))
}

func (s *Server) handleTunnelDelete(w http.ResponseWriter, r *http.Request) {
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
	if _, err := s.getTunnelByID(id); err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "隧道不存在"))
		return
	}
	var forwardCount int
	_ = s.db.QueryRow(`SELECT COUNT(1) FROM forward WHERE tunnel_id = ?`, id).Scan(&forwardCount)
	if forwardCount > 0 {
		writeJSON(w, http.StatusOK, errResp(-1, fmt.Sprintf("该隧道还有%d个转发在使用，请先删除相关转发", forwardCount)))
		return
	}
	var permissionCount int
	_ = s.db.QueryRow(`SELECT COUNT(1) FROM user_tunnel WHERE tunnel_id = ?`, id).Scan(&permissionCount)
	if permissionCount > 0 {
		writeJSON(w, http.StatusOK, errResp(-1, fmt.Sprintf("该隧道还有%d个用户权限关联，请先取消用户权限分配", permissionCount)))
		return
	}
	if _, err := s.db.Exec(`DELETE FROM tunnel WHERE id = ?`, id); err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "隧道删除失败"))
		return
	}
	writeJSON(w, http.StatusOK, ok("隧道删除成功"))
}

func (s *Server) handleUserTunnelAssign(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if _, err := s.requireAdmin(r); err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	var req UserTunnelDTO
	if err := s.decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(-1, "请求参数错误"))
		return
	}
	if _, err := s.getUserTunnelByUserAndTunnel(req.UserID, req.TunnelID); err == nil {
		writeJSON(w, http.StatusOK, errResp(-1, "该用户已拥有此隧道权限"))
		return
	}
	_, err := s.db.Exec(`
		INSERT INTO user_tunnel (user_id, tunnel_id, speed_id, num, flow, in_flow, out_flow, flow_reset_time, exp_time, status)
		VALUES (?, ?, ?, ?, ?, 0, 0, ?, ?, 1)`,
		req.UserID, req.TunnelID, req.SpeedID, req.Num, req.Flow, req.FlowResetTime, req.ExpTime,
	)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "用户隧道权限分配失败"))
		return
	}
	writeJSON(w, http.StatusOK, ok("用户隧道权限分配成功"))
}

func (s *Server) handleUserTunnelList(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if _, err := s.requireAdmin(r); err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	var req UserTunnelQueryDTO
	if err := s.decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(-1, "请求参数错误"))
		return
	}
	items, err := s.listUserTunnels(req.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(-2, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, ok(items))
}

func (s *Server) handleUserTunnelRemove(w http.ResponseWriter, r *http.Request) {
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
	item, err := s.getUserTunnelByID(id)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "未找到对应的用户隧道权限记录"))
		return
	}
	forwards, _ := s.listForwardsByUserAndTunnel(item.UserID, item.TunnelID)
	for _, forward := range forwards {
		_ = s.cleanupForwardServices(forward)
		_, _ = s.db.Exec(`DELETE FROM forward WHERE id = ?`, forward.ID)
	}
	if _, err := s.db.Exec(`DELETE FROM user_tunnel WHERE id = ?`, id); err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "用户隧道权限删除失败"))
		return
	}
	writeJSON(w, http.StatusOK, ok("用户隧道权限删除成功"))
}

func (s *Server) handleUserTunnelUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if _, err := s.requireAdmin(r); err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	var req UserTunnelUpdateDTO
	if err := s.decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(-1, "请求参数错误"))
		return
	}
	item, err := s.getUserTunnelByID(req.ID)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "用户隧道权限不存在"))
		return
	}
	speedChanged := (item.SpeedID == nil && req.SpeedID != nil) ||
		(item.SpeedID != nil && req.SpeedID == nil) ||
		(item.SpeedID != nil && req.SpeedID != nil && *item.SpeedID != *req.SpeedID)

	_, err = s.db.Exec(`
		UPDATE user_tunnel
		SET flow = ?, num = ?, flow_reset_time = ?, exp_time = ?, speed_id = ?, status = ?
		WHERE id = ?`,
		req.Flow, req.Num, req.FlowResetTime, req.ExpTime, req.SpeedID, req.Status, req.ID,
	)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "用户隧道权限更新失败"))
		return
	}
	if speedChanged {
		forwards, _ := s.listForwardsByUserAndTunnel(item.UserID, item.TunnelID)
		for _, forward := range forwards {
			_ = s.syncForwardRuntime(forward.ID)
		}
	}
	writeJSON(w, http.StatusOK, ok("用户隧道权限更新成功"))
}

func (s *Server) handleUserAccessibleTunnels(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	claims, err := s.requireAuth(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	userID, _ := strconv.ParseInt(claims.Sub, 10, 64)
	items, err := s.listAccessibleTunnels(userID, claims.RoleID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(-2, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, ok(items))
}

func (s *Server) handleTunnelDiagnose(w http.ResponseWriter, r *http.Request) {
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
	tunnelID := mapInt64(params["tunnelId"])
	tunnel, err := s.getTunnelByID(tunnelID)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "隧道不存在"))
		return
	}
	inNode, err := s.getNodeByID(tunnel.InNodeID)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "入口节点不存在"))
		return
	}

	results := make([]map[string]interface{}, 0, 2)
	if tunnel.Type == 1 {
		results = append(results, s.performTCPDiagnosis(inNode, "www.google.com", 443, "入口->外网"))
	} else {
		outNode, err := s.getNodeByID(tunnel.OutNodeID)
		if err != nil {
			writeJSON(w, http.StatusOK, errResp(-1, "出口节点不存在"))
			return
		}
		outPort := 22
		forwards, _ := s.listForwardsByTunnel(tunnel.ID)
		if len(forwards) > 0 && forwards[0].OutPort > 0 {
			outPort = forwards[0].OutPort
		}
		results = append(results, s.performTCPDiagnosis(inNode, outNode.ServerIP, outPort, "入口->出口"))
		results = append(results, s.performTCPDiagnosis(outNode, "www.google.com", 443, "出口->外网"))
	}

	writeJSON(w, http.StatusOK, ok(map[string]interface{}{
		"tunnelId":   tunnel.ID,
		"tunnelName": tunnel.Name,
		"tunnelType": map[bool]string{true: "端口转发", false: "隧道转发"}[tunnel.Type == 1],
		"results":    results,
		"timestamp":  time.Now().UnixMilli(),
	}))
}

func (s *Server) performTCPDiagnosis(node Node, targetIP string, targetPort int, description string) map[string]interface{} {
	result := map[string]interface{}{
		"nodeId":      node.ID,
		"nodeName":    node.Name,
		"targetIp":    targetIP,
		"targetPort":  targetPort,
		"description": description,
		"timestamp":   time.Now().UnixMilli(),
	}
	response := s.sendNodeCommand(node.ID, "TcpPing", map[string]interface{}{
		"ip":      targetIP,
		"port":    targetPort,
		"count":   2,
		"timeout": 3000,
	})
	if response.Msg != "OK" {
		result["success"] = false
		result["message"] = response.Msg
		result["averageTime"] = -1
		result["packetLoss"] = 100
		return result
	}
	result["success"] = true
	if data, ok := response.Data.(map[string]interface{}); ok {
		if value, exists := data["averageTime"]; exists {
			result["averageTime"] = value
		}
		if value, exists := data["packetLoss"]; exists {
			result["packetLoss"] = value
		}
		if value, exists := data["errorMessage"]; exists {
			result["message"] = value
		} else {
			result["message"] = "TCP连接成功"
		}
	}
	return result
}
