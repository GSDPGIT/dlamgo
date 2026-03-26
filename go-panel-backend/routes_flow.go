package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (s *Server) handleFlowConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	secret := s.extractNodeSecret(r)
	if secret == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if _, err := s.getNodeBySecret(secret); err != nil {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleFlowUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	secret := s.extractNodeSecret(r)
	if secret == "" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
		return
	}
	node, err := s.getNodeBySecret(secret)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
		return
	}
	body, err := s.decodeNodePayload(secret, raw)
	if err != nil {
		body = raw
	}
	var flow FlowDTO
	if err := json.Unmarshal(body, &flow); err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
		return
	}
	if flow.N == "web_api" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
		return
	}
	parts := strings.Split(flow.N, "_")
	if len(parts) != 3 {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
		return
	}
	forwardID, _ := strconv.ParseInt(parts[0], 10, 64)
	userID, _ := strconv.ParseInt(parts[1], 10, 64)
	userTunnelID, _ := strconv.ParseInt(parts[2], 10, 64)

	forward, err := s.getForwardByID(forwardID)
	if err != nil || forward.UserID != userID {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
		return
	}
	tunnel, err := s.getTunnelByID(forward.TunnelID)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
		return
	}
	if node.ID != tunnel.InNodeID && node.ID != tunnel.OutNodeID {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
		return
	}
	billedU, billedD := s.billFlow(tunnel, flow.U, flow.D)

	tx, err := s.db.Begin()
	if err == nil {
		_, _ = tx.Exec(`UPDATE forward SET in_flow = in_flow + ?, out_flow = out_flow + ?, updated_time = ? WHERE id = ?`, billedD, billedU, time.Now().UnixMilli(), forward.ID)
		_, _ = tx.Exec(`UPDATE user SET in_flow = in_flow + ?, out_flow = out_flow + ?, updated_time = ? WHERE id = ?`, billedD, billedU, time.Now().UnixMilli(), userID)
		if userTunnelID > 0 {
			_, _ = tx.Exec(`UPDATE user_tunnel SET in_flow = in_flow + ?, out_flow = out_flow + ? WHERE id = ?`, billedD, billedU, userTunnelID)
		}
		_ = tx.Commit()
	}

	s.enforceForwardQuotas(userID, forward, tunnel, userTunnelID)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleFlowTest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("test"))
}

func (s *Server) extractNodeSecret(r *http.Request) string {
	secret := strings.TrimSpace(r.Header.Get("X-Node-Secret"))
	if secret == "" {
		secret = strings.TrimSpace(r.URL.Query().Get("secret"))
	}
	return secret
}

func (s *Server) decodeNodePayload(secret string, raw []byte) ([]byte, error) {
	var envelope EncryptedEnvelope
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Encrypted && envelope.Data != "" {
		return decryptSecretPayload(secret, envelope.Data)
	}
	return raw, nil
}

func (s *Server) billFlow(tunnel Tunnel, u, d int64) (int64, int64) {
	ratio := tunnel.TrafficRatio
	if ratio <= 0 {
		ratio = 1
	}
	multiplier := int64(tunnel.Flow)
	if multiplier <= 0 {
		multiplier = 1
	}
	return int64(float64(u)*ratio) * multiplier, int64(float64(d)*ratio) * multiplier
}

func (s *Server) enforceForwardQuotas(userID int64, forward Forward, tunnel Tunnel, userTunnelID int64) {
	user, err := s.getUserByID(userID)
	if err != nil {
		return
	}
	if user.Status != 1 || (user.ExpTime > 0 && user.ExpTime <= time.Now().UnixMilli()) || exceedsUserFlow(user) {
		forwards, _ := s.listForwardsByUser(userID)
		for _, item := range forwards {
			_ = s.pauseForwardRuntime(item)
		}
		return
	}
	if userTunnelID <= 0 {
		return
	}
	userTunnel, err := s.getUserTunnelByID(userTunnelID)
	if err != nil {
		return
	}
	if userTunnel.Status != 1 || (userTunnel.ExpTime > 0 && userTunnel.ExpTime <= time.Now().UnixMilli()) || exceedsUserTunnelFlow(userTunnel) {
		forwards, _ := s.listForwardsByUserAndTunnel(userID, tunnel.ID)
		for _, item := range forwards {
			_ = s.pauseForwardRuntime(item)
		}
	}
}

func exceedsUserFlow(user User) bool {
	if user.Flow == 99999 {
		return false
	}
	return user.Flow*1024*1024*1024 <= user.InFlow+user.OutFlow
}

func exceedsUserTunnelFlow(userTunnel UserTunnel) bool {
	if userTunnel.Flow == 99999 {
		return false
	}
	return userTunnel.Flow*1024*1024*1024 <= userTunnel.InFlow+userTunnel.OutFlow
}

func (s *Server) pauseForwardRuntime(forward Forward) error {
	tunnel, err := s.getTunnelByID(forward.TunnelID)
	if err != nil {
		return err
	}
	userTunnel, _ := s.findUserTunnelOrZero(forward.UserID, forward.TunnelID)
	serviceName := buildServiceName(forward.ID, forward.UserID, userTunnel.ID)
	_ = s.sendNodeCommand(tunnel.InNodeID, "PauseService", deleteServicesPayload(serviceName+"_tcp", serviceName+"_udp"))
	if tunnel.Type == 2 {
		_ = s.sendNodeCommand(tunnel.OutNodeID, "PauseService", deleteServicesPayload(serviceName+"_tls"))
	}
	_, _ = s.db.Exec(`UPDATE forward SET status = 0, updated_time = ? WHERE id = ?`, time.Now().UnixMilli(), forward.ID)
	return nil
}

func (s *Server) runHourlyStatsJob() {
	for {
		next := time.Now().Add(time.Hour).Truncate(time.Hour)
		time.Sleep(time.Until(next))
		_ = s.captureHourlyStatistics()
	}
}

func (s *Server) captureHourlyStatistics() error {
	now := time.Now()
	_ = s.deleteExpiredStatistics(now.Add(-48 * time.Hour).UnixMilli())
	rows, err := s.db.Query(`SELECT id, user, pwd, role_id, exp_time, flow, in_flow, out_flow, flow_reset_time, num, subscription_token, created_time, updated_time, status FROM user ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var snapshots []StatisticsFlow
	for rows.Next() {
		user, err := s.scanUser(rows)
		if err != nil {
			return err
		}
		currentTotal := user.InFlow + user.OutFlow
		lastRecords, _ := s.listStatisticsForUser(user.ID, 1)
		increment := currentTotal
		if len(lastRecords) > 0 {
			increment = currentTotal - lastRecords[0].TotalFlow
			if increment < 0 {
				increment = currentTotal
			}
		}
		snapshots = append(snapshots, StatisticsFlow{
			UserID:      user.ID,
			Flow:        increment,
			TotalFlow:   currentTotal,
			Time:        now.Format("15:00"),
			CreatedTime: now.UnixMilli(),
		})
	}
	if len(snapshots) == 0 {
		return nil
	}
	return s.insertStatisticsFlows(snapshots)
}

func (s *Server) runDailyResetJob() {
	for {
		next := time.Now().Truncate(24 * time.Hour).Add(24*time.Hour + 5*time.Second)
		time.Sleep(time.Until(next))
		s.resetFlowAndExpireAccounts()
	}
}

func (s *Server) resetFlowAndExpireAccounts() {
	today := time.Now()
	currentDay := int64(today.Day())
	lastDay := int64(time.Date(today.Year(), today.Month()+1, 0, 0, 0, 0, 0, today.Location()).Day())

	_, _ = s.db.Exec(`UPDATE user SET in_flow = 0, out_flow = 0, updated_time = ? WHERE flow_reset_time <> 0 AND (flow_reset_time = ? OR (? = ? AND flow_reset_time > ?))`, today.UnixMilli(), currentDay, currentDay, lastDay, lastDay)
	_, _ = s.db.Exec(`UPDATE user_tunnel SET in_flow = 0, out_flow = 0 WHERE flow_reset_time <> 0 AND (flow_reset_time = ? OR (? = ? AND flow_reset_time > ?))`, currentDay, currentDay, lastDay, lastDay)

	rows, err := s.db.Query(`SELECT id, user_id, user_name, name, tunnel_id, in_port, out_port, remote_addr, strategy, interface_name, in_flow, out_flow, created_time, updated_time, status, inx, '' as tunnel_name, '' as in_ip, '' as out_ip, 0 as type, '' as protocol FROM forward WHERE status = 1 ORDER BY id`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		forward, err := s.scanForward(rows)
		if err != nil {
			continue
		}
		tunnel, err := s.getTunnelByID(forward.TunnelID)
		if err != nil {
			continue
		}
		user, err := s.getUserByID(forward.UserID)
		if err != nil {
			continue
		}
		if user.Status != 1 || (user.ExpTime > 0 && user.ExpTime <= today.UnixMilli()) {
			_ = s.pauseForwardRuntime(forward)
			_, _ = s.db.Exec(`UPDATE user SET status = 0, updated_time = ? WHERE id = ?`, today.UnixMilli(), user.ID)
			continue
		}
		userTunnel, err := s.getUserTunnelByUserAndTunnel(forward.UserID, tunnel.ID)
		if err == nil {
			if userTunnel.Status != 1 || (userTunnel.ExpTime > 0 && userTunnel.ExpTime <= today.UnixMilli()) {
				_ = s.pauseForwardRuntime(forward)
				_, _ = s.db.Exec(`UPDATE user_tunnel SET status = 0 WHERE id = ?`, userTunnel.ID)
			}
		}
	}
}
