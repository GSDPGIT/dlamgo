package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (s *Server) handleForwardCreate(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	claims, err := s.requireAuth(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	var req ForwardDTO
	if err := s.decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(-1, "请求参数错误"))
		return
	}
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.RemoteAddr) == "" {
		writeJSON(w, http.StatusOK, errResp(-1, "转发名称和远程地址不能为空"))
		return
	}
	ownerID, _ := strconv.ParseInt(claims.Sub, 10, 64)
	owner, err := s.getUserByID(ownerID)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "用户不存在"))
		return
	}
	tunnel, err := s.getTunnelByID(req.TunnelID)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "隧道不存在"))
		return
	}
	if tunnel.Status != 1 {
		writeJSON(w, http.StatusOK, errResp(-1, "隧道已禁用，无法创建转发"))
		return
	}

	userTunnel, limiterID, errRespText := s.checkForwardPermissions(owner, claims.RoleID, tunnel, 0)
	if errRespText != "" {
		writeJSON(w, http.StatusOK, errResp(-1, errRespText))
		return
	}
	inPort, outPort, errRespText := s.allocateForwardPorts(tunnel, req.InPort, 0)
	if errRespText != "" {
		writeJSON(w, http.StatusOK, errResp(-1, errRespText))
		return
	}

	now := time.Now().UnixMilli()
	result, err := s.db.Exec(`
		INSERT INTO forward (user_id, user_name, name, tunnel_id, in_port, out_port, remote_addr, strategy, interface_name, in_flow, out_flow, created_time, updated_time, status, inx)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, ?, ?, 1, 0)`,
		owner.ID,
		owner.User,
		strings.TrimSpace(req.Name),
		tunnel.ID,
		inPort,
		outPort,
		normalizeRemoteAddr(req.RemoteAddr),
		defaultStrategy(req.Strategy),
		strings.TrimSpace(req.InterfaceName),
		now,
		now,
	)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "端口转发创建失败"))
		return
	}
	forwardID, _ := result.LastInsertId()
	forward, err := s.getForwardByID(forwardID)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "端口转发创建失败"))
		return
	}
	if err := s.provisionForward(forward, tunnel, userTunnel, limiterID, false); err != nil {
		_, _ = s.db.Exec(`DELETE FROM forward WHERE id = ?`, forwardID)
		writeJSON(w, http.StatusOK, errResp(-1, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, okNoData())
}

func (s *Server) handleForwardList(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	claims, err := s.requireAuth(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	var userID *int64
	if claims.RoleID != 0 {
		id, _ := strconv.ParseInt(claims.Sub, 10, 64)
		userID = &id
	}
	items, err := s.listForwards(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(-2, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, ok(items))
}

func (s *Server) handleForwardUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	claims, err := s.requireAuth(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	var req ForwardUpdateDTO
	if err := s.decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(-1, "请求参数错误"))
		return
	}
	existing, err := s.getForwardByID(req.ID)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "转发不存在"))
		return
	}
	if claims.RoleID != 0 {
		currentID, _ := strconv.ParseInt(claims.Sub, 10, 64)
		if existing.UserID != currentID {
			writeJSON(w, http.StatusOK, errResp(-1, "转发不存在"))
			return
		}
	}
	owner, err := s.getUserByID(existing.UserID)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "用户不存在"))
		return
	}
	newTunnel, err := s.getTunnelByID(req.TunnelID)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "隧道不存在"))
		return
	}
	if newTunnel.Status != 1 {
		writeJSON(w, http.StatusOK, errResp(-1, "隧道已禁用，无法更新转发"))
		return
	}
	userTunnel, limiterID, errRespText := s.checkForwardPermissions(owner, claims.RoleID, newTunnel, existing.ID)
	if errRespText != "" {
		writeJSON(w, http.StatusOK, errResp(-1, errRespText))
		return
	}

	oldTunnel, _ := s.getTunnelByID(existing.TunnelID)
	if req.InterfaceName == nil {
		req.InterfaceName = &existing.InterfaceName
	}
	inPort, outPort, errRespText := s.allocateForwardPorts(newTunnel, req.InPort, existing.ID)
	if errRespText != "" {
		writeJSON(w, http.StatusOK, errResp(-1, errRespText))
		return
	}

	if oldTunnel.ID != 0 && oldTunnel.ID != newTunnel.ID {
		_ = s.cleanupForwardServices(existing)
	}

	_, err = s.db.Exec(`
		UPDATE forward
		SET name = ?, tunnel_id = ?, in_port = ?, out_port = ?, remote_addr = ?, strategy = ?, interface_name = ?, updated_time = ?
		WHERE id = ?`,
		strings.TrimSpace(req.Name),
		newTunnel.ID,
		inPort,
		outPort,
		normalizeRemoteAddr(req.RemoteAddr),
		defaultStrategy(req.Strategy),
		strings.TrimSpace(*req.InterfaceName),
		time.Now().UnixMilli(),
		existing.ID,
	)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "端口转发更新失败"))
		return
	}
	if err := s.syncForwardRuntime(existing.ID); err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, err.Error()))
		return
	}
	if limiterID != nil || userTunnel.ID >= 0 {
	}
	writeJSON(w, http.StatusOK, ok("端口转发更新成功"))
}

func (s *Server) handleForwardDelete(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	claims, err := s.requireAuth(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	var params map[string]interface{}
	if err := s.decodeJSON(r, &params); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(-1, "请求参数错误"))
		return
	}
	forward, err := s.getForwardByID(mapInt64(params["id"]))
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "端口转发不存在"))
		return
	}
	if claims.RoleID != 0 {
		userID, _ := strconv.ParseInt(claims.Sub, 10, 64)
		if forward.UserID != userID {
			writeJSON(w, http.StatusOK, errResp(-1, "端口转发不存在"))
			return
		}
	}
	_ = s.cleanupForwardServices(forward)
	if _, err := s.db.Exec(`DELETE FROM forward WHERE id = ?`, forward.ID); err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "端口转发删除失败"))
		return
	}
	writeJSON(w, http.StatusOK, ok("端口转发删除成功"))
}

func (s *Server) handleForwardForceDelete(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	claims, err := s.requireAuth(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	var params map[string]interface{}
	if err := s.decodeJSON(r, &params); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(-1, "请求参数错误"))
		return
	}
	forward, err := s.getForwardByID(mapInt64(params["id"]))
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "端口转发不存在"))
		return
	}
	if claims.RoleID != 0 {
		userID, _ := strconv.ParseInt(claims.Sub, 10, 64)
		if forward.UserID != userID {
			writeJSON(w, http.StatusOK, errResp(-1, "端口转发不存在"))
			return
		}
	}
	if _, err := s.db.Exec(`DELETE FROM forward WHERE id = ?`, forward.ID); err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "端口转发强制删除失败"))
		return
	}
	writeJSON(w, http.StatusOK, ok("端口转发强制删除成功"))
}

func (s *Server) handleForwardPause(w http.ResponseWriter, r *http.Request) {
	s.handleForwardStateChange(w, r, 0)
}

func (s *Server) handleForwardResume(w http.ResponseWriter, r *http.Request) {
	s.handleForwardStateChange(w, r, 1)
}

func (s *Server) handleForwardStateChange(w http.ResponseWriter, r *http.Request, status int) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	claims, err := s.requireAuth(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	var params map[string]interface{}
	if err := s.decodeJSON(r, &params); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(-1, "请求参数错误"))
		return
	}
	forward, err := s.getForwardByID(mapInt64(params["id"]))
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "转发不存在"))
		return
	}
	if claims.RoleID != 0 {
		userID, _ := strconv.ParseInt(claims.Sub, 10, 64)
		if forward.UserID != userID {
			writeJSON(w, http.StatusOK, errResp(-1, "转发不存在"))
			return
		}
	}
	tunnel, err := s.getTunnelByID(forward.TunnelID)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "隧道不存在"))
		return
	}
	userTunnel, _ := s.findUserTunnelOrZero(forward.UserID, forward.TunnelID)
	serviceName := buildServiceName(forward.ID, forward.UserID, userTunnel.ID)

	if status == 1 {
		owner, _ := s.getUserByID(forward.UserID)
		_, _, errText := s.checkForwardPermissions(owner, claims.RoleID, tunnel, forward.ID)
		if errText != "" {
			writeJSON(w, http.StatusOK, errResp(-1, errText))
			return
		}
		if result := s.sendNodeCommand(tunnel.InNodeID, "ResumeService", deleteServicesPayload(serviceName+"_tcp", serviceName+"_udp")); result.Msg != "OK" {
			writeJSON(w, http.StatusOK, errResp(-1, result.Msg))
			return
		}
		if tunnel.Type == 2 {
			if result := s.sendNodeCommand(tunnel.OutNodeID, "ResumeService", deleteServicesPayload(serviceName+"_tls")); result.Msg != "OK" {
				writeJSON(w, http.StatusOK, errResp(-1, result.Msg))
				return
			}
		}
	} else {
		_ = s.sendNodeCommand(tunnel.InNodeID, "PauseService", deleteServicesPayload(serviceName+"_tcp", serviceName+"_udp"))
		if tunnel.Type == 2 {
			_ = s.sendNodeCommand(tunnel.OutNodeID, "PauseService", deleteServicesPayload(serviceName+"_tls"))
		}
	}
	_, _ = s.db.Exec(`UPDATE forward SET status = ?, updated_time = ? WHERE id = ?`, status, time.Now().UnixMilli(), forward.ID)
	writeJSON(w, http.StatusOK, ok("OK"))
}

func (s *Server) handleForwardDiagnose(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	claims, err := s.requireAuth(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	var params map[string]interface{}
	if err := s.decodeJSON(r, &params); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(-1, "请求参数错误"))
		return
	}
	forwardID := mapInt64(params["forwardId"])
	forward, err := s.getForwardByID(forwardID)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "转发不存在"))
		return
	}
	if claims.RoleID != 0 {
		userID, _ := strconv.ParseInt(claims.Sub, 10, 64)
		if forward.UserID != userID {
			writeJSON(w, http.StatusOK, errResp(-1, "转发不存在"))
			return
		}
	}
	tunnel, err := s.getTunnelByID(forward.TunnelID)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "隧道不存在"))
		return
	}
	inNode, err := s.getNodeByID(tunnel.InNodeID)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "入口节点不存在"))
		return
	}
	results := make([]map[string]interface{}, 0, 4)
	if tunnel.Type == 1 {
		for _, address := range splitAddressList(forward.RemoteAddr) {
			host, port := parseAddressHostPort(address)
			if host == "" || port == 0 {
				continue
			}
			results = append(results, s.performTCPDiagnosis(inNode, host, port, "转发->目标"))
		}
	} else {
		outNode, err := s.getNodeByID(tunnel.OutNodeID)
		if err != nil {
			writeJSON(w, http.StatusOK, errResp(-1, "出口节点不存在"))
			return
		}
		results = append(results, s.performTCPDiagnosis(inNode, outNode.ServerIP, forward.OutPort, "入口->出口"))
		for _, address := range splitAddressList(forward.RemoteAddr) {
			host, port := parseAddressHostPort(address)
			if host == "" || port == 0 {
				continue
			}
			results = append(results, s.performTCPDiagnosis(outNode, host, port, "出口->目标"))
		}
	}
	writeJSON(w, http.StatusOK, ok(map[string]interface{}{
		"forwardId":   forward.ID,
		"forwardName": forward.Name,
		"results":     results,
		"timestamp":   time.Now().UnixMilli(),
	}))
}

func (s *Server) handleForwardUpdateOrder(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	claims, err := s.requireAuth(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	var params map[string]interface{}
	if err := s.decodeJSON(r, &params); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(-1, "请求参数错误"))
		return
	}
	items, hasItems := params["forwards"].([]interface{})
	if !hasItems || len(items) == 0 {
		writeJSON(w, http.StatusOK, errResp(-1, "forwards参数不能为空"))
		return
	}
	userID, _ := strconv.ParseInt(claims.Sub, 10, 64)
	tx, err := s.db.Begin()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(-2, err.Error()))
		return
	}
	defer func() { _ = tx.Rollback() }()
	for _, item := range items {
		record, _ := item.(map[string]interface{})
		id := mapInt64(record["id"])
		inx := mapInt64(record["inx"])
		forward, err := s.getForwardByID(id)
		if err != nil {
			writeJSON(w, http.StatusOK, errResp(-1, "转发不存在"))
			return
		}
		if claims.RoleID != 0 && forward.UserID != userID {
			writeJSON(w, http.StatusOK, errResp(-1, "只能更新自己的转发排序"))
			return
		}
		if _, err := tx.Exec(`UPDATE forward SET inx = ?, updated_time = ? WHERE id = ?`, inx, time.Now().UnixMilli(), id); err != nil {
			writeJSON(w, http.StatusInternalServerError, errResp(-2, err.Error()))
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(-2, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, ok("排序更新成功"))
}

func (s *Server) syncForwardRuntime(forwardID int64) error {
	forward, err := s.getForwardByID(forwardID)
	if err != nil {
		return err
	}
	tunnel, err := s.getTunnelByID(forward.TunnelID)
	if err != nil {
		return err
	}
	userTunnel, _ := s.findUserTunnelOrZero(forward.UserID, forward.TunnelID)
	var limiterID *int64
	if userTunnel.SpeedID != nil {
		limiterID = userTunnel.SpeedID
	}
	return s.provisionForward(forward, tunnel, userTunnel, limiterID, true)
}

func (s *Server) provisionForward(forward Forward, tunnel Tunnel, userTunnel UserTunnel, limiterID *int64, update bool) error {
	serviceName := buildServiceName(forward.ID, forward.UserID, userTunnel.ID)
	mainPayload := createMainServices(serviceName, forward.InPort, limiterID, forward.RemoteAddr, tunnel.Type, tunnel, forward.Strategy, forward.InterfaceName)
	if tunnel.Type == 2 {
		chainPayload := createChain(serviceName, formatRemoteNodeAddress(tunnel.OutIP, forward.OutPort), tunnel.Protocol, tunnel.InterfaceName)
		command := "AddChains"
		payload := interface{}(chainPayload)
		if update {
			command = "UpdateChains"
			payload = map[string]interface{}{"chain": serviceName + "_chains", "data": chainPayload}
		}
		result := s.sendNodeCommand(tunnel.InNodeID, command, payload)
		if update && strings.Contains(strings.ToLower(result.Msg), "not found") {
			result = s.sendNodeCommand(tunnel.InNodeID, "AddChains", chainPayload)
		}
		if result.Msg != "OK" {
			_, _ = s.db.Exec(`UPDATE forward SET status = -1 WHERE id = ?`, forward.ID)
			return errors.New(result.Msg)
		}

		remotePayload := createRemoteService(serviceName, forward.OutPort, forward.RemoteAddr, tunnel.Protocol, forward.Strategy, forward.InterfaceName)
		command = "AddService"
		payload = remotePayload
		if update {
			command = "UpdateService"
		}
		result = s.sendNodeCommand(tunnel.OutNodeID, command, payload)
		if update && strings.Contains(strings.ToLower(result.Msg), "not found") {
			result = s.sendNodeCommand(tunnel.OutNodeID, "AddService", remotePayload)
		}
		if result.Msg != "OK" {
			_, _ = s.db.Exec(`UPDATE forward SET status = -1 WHERE id = ?`, forward.ID)
			return errors.New(result.Msg)
		}
	}

	command := "AddService"
	if update {
		command = "UpdateService"
	}
	result := s.sendNodeCommand(tunnel.InNodeID, command, mainPayload)
	if update && strings.Contains(strings.ToLower(result.Msg), "not found") {
		result = s.sendNodeCommand(tunnel.InNodeID, "AddService", mainPayload)
	}
	if result.Msg != "OK" {
		_, _ = s.db.Exec(`UPDATE forward SET status = -1 WHERE id = ?`, forward.ID)
		return errors.New(result.Msg)
	}
	_, _ = s.db.Exec(`UPDATE forward SET status = 1, updated_time = ? WHERE id = ?`, time.Now().UnixMilli(), forward.ID)
	return nil
}

func (s *Server) checkForwardPermissions(owner User, roleID int, tunnel Tunnel, excludeForwardID int64) (UserTunnel, *int64, string) {
	if roleID == 0 && owner.RoleID == 0 {
		return UserTunnel{ID: 0}, nil, ""
	}
	if owner.Status == 0 || (owner.ExpTime > 0 && owner.ExpTime <= time.Now().UnixMilli()) {
		return UserTunnel{}, nil, "当前账号已到期或被禁用"
	}
	if owner.Flow != 99999 && owner.Flow*1024*1024*1024 <= owner.InFlow+owner.OutFlow {
		return UserTunnel{}, nil, "用户总流量已用完"
	}
	userTunnel, err := s.getUserTunnelByUserAndTunnel(owner.ID, tunnel.ID)
	if err != nil {
		return UserTunnel{}, nil, "你没有该隧道权限"
	}
	if userTunnel.Status != 1 {
		return UserTunnel{}, nil, "隧道被禁用"
	}
	if userTunnel.ExpTime > 0 && userTunnel.ExpTime <= time.Now().UnixMilli() {
		return UserTunnel{}, nil, "该隧道权限已到期"
	}
	if userTunnel.Flow != 99999 && userTunnel.Flow*1024*1024*1024 <= userTunnel.InFlow+userTunnel.OutFlow {
		return UserTunnel{}, nil, "该隧道流量已用完"
	}
	var count int64
	_ = s.db.QueryRow(`SELECT COUNT(1) FROM forward WHERE user_id = ?`+excludeClause(excludeForwardID), append([]interface{}{owner.ID}, excludeArgs(excludeForwardID)...)...).Scan(&count)
	if owner.Num != 99999 && count >= int64(owner.Num) {
		return UserTunnel{}, nil, fmt.Sprintf("用户总转发数量已达上限，当前限制：%d个", owner.Num)
	}
	_ = s.db.QueryRow(`SELECT COUNT(1) FROM forward WHERE user_id = ? AND tunnel_id = ?`+excludeClause(excludeForwardID), append([]interface{}{owner.ID, tunnel.ID}, excludeArgs(excludeForwardID)...)...).Scan(&count)
	if userTunnel.Num != 99999 && count >= int64(userTunnel.Num) {
		return UserTunnel{}, nil, fmt.Sprintf("该隧道转发数量已达上限，当前限制：%d个", userTunnel.Num)
	}
	return userTunnel, userTunnel.SpeedID, ""
}

func (s *Server) allocateForwardPorts(tunnel Tunnel, preferredInPort *int, excludeForwardID int64) (int, int, string) {
	inNode, err := s.getNodeByID(tunnel.InNodeID)
	if err != nil {
		return 0, 0, "入口节点不存在"
	}
	usedIn, err := s.usedPortsForNode(inNode.ID, true, excludeForwardID)
	if err != nil {
		return 0, 0, "端口分配失败"
	}
	inPort := 0
	if preferredInPort != nil {
		if *preferredInPort < inNode.PortSta || *preferredInPort > inNode.PortEnd {
			return 0, 0, fmt.Sprintf("指定的入口端口%d不在允许范围内", *preferredInPort)
		}
		if _, exists := usedIn[*preferredInPort]; exists {
			return 0, 0, fmt.Sprintf("指定的入口端口%d已被占用", *preferredInPort)
		}
		inPort = *preferredInPort
	} else {
		for port := inNode.PortSta; port <= inNode.PortEnd; port++ {
			if _, exists := usedIn[port]; !exists {
				inPort = port
				break
			}
		}
	}
	if inPort == 0 {
		return 0, 0, "隧道入口端口已满，无法分配新端口"
	}
	if tunnel.Type != 2 {
		return inPort, 0, ""
	}
	outNode, err := s.getNodeByID(tunnel.OutNodeID)
	if err != nil {
		return 0, 0, "出口节点不存在"
	}
	usedOut, err := s.usedPortsForNode(outNode.ID, false, excludeForwardID)
	if err != nil {
		return 0, 0, "端口分配失败"
	}
	for port := outNode.PortSta; port <= outNode.PortEnd; port++ {
		if _, exists := usedOut[port]; !exists {
			return inPort, port, ""
		}
	}
	return 0, 0, "隧道出口端口已满，无法分配新端口"
}

func (s *Server) usedPortsForNode(nodeID int64, inbound bool, excludeForwardID int64) (map[int]struct{}, error) {
	query := `
		SELECT f.in_port, f.out_port
		FROM forward f
		INNER JOIN tunnel t ON t.id = f.tunnel_id
		WHERE ` + map[bool]string{true: "t.in_node_id = ?", false: "t.out_node_id = ?"}[inbound]
	args := []interface{}{nodeID}
	if excludeForwardID > 0 {
		query += ` AND f.id <> ?`
		args = append(args, excludeForwardID)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	used := map[int]struct{}{}
	for rows.Next() {
		var inPort, outPort int
		if err := rows.Scan(&inPort, &outPort); err != nil {
			return nil, err
		}
		if inbound {
			used[inPort] = struct{}{}
		} else if outPort > 0 {
			used[outPort] = struct{}{}
		}
	}
	return used, rows.Err()
}

func excludeClause(excludeForwardID int64) string {
	if excludeForwardID <= 0 {
		return ""
	}
	return " AND id <> ?"
}

func excludeArgs(excludeForwardID int64) []interface{} {
	if excludeForwardID <= 0 {
		return nil
	}
	return []interface{}{excludeForwardID}
}

func normalizeRemoteAddr(raw string) string {
	lines := strings.Split(raw, "\n")
	values := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			values = append(values, line)
		}
	}
	return strings.Join(values, ",")
}

func defaultStrategy(strategy string) string {
	if strings.TrimSpace(strategy) == "" {
		return "fifo"
	}
	return strings.TrimSpace(strategy)
}

func formatRemoteNodeAddress(host string, port int) string {
	if strings.Count(host, ":") >= 2 && !strings.HasPrefix(host, "[") {
		return "[" + host + "]:" + strconv.Itoa(port)
	}
	return host + ":" + strconv.Itoa(port)
}

func parseAddressHostPort(address string) (string, int) {
	address = strings.TrimSpace(address)
	if address == "" {
		return "", 0
	}
	host, port, err := net.SplitHostPort(address)
	if err == nil {
		portInt, _ := strconv.Atoi(port)
		return strings.Trim(host, "[]"), portInt
	}
	lastColon := strings.LastIndex(address, ":")
	if lastColon <= 0 {
		return "", 0
	}
	portInt, _ := strconv.Atoi(address[lastColon+1:])
	return strings.Trim(address[:lastColon], "[]"), portInt
}
