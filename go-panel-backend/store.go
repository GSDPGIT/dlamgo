package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

func (s *Server) initSchema() error {
	stmts := []string{
		`PRAGMA journal_mode=WAL;`,
		`PRAGMA foreign_keys=ON;`,
		`CREATE TABLE IF NOT EXISTS user (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user TEXT NOT NULL UNIQUE,
			pwd TEXT NOT NULL,
			role_id INTEGER NOT NULL DEFAULT 1,
			exp_time INTEGER NOT NULL DEFAULT 0,
			flow INTEGER NOT NULL DEFAULT 0,
			in_flow INTEGER NOT NULL DEFAULT 0,
			out_flow INTEGER NOT NULL DEFAULT 0,
			flow_reset_time INTEGER NOT NULL DEFAULT 0,
			num INTEGER NOT NULL DEFAULT 0,
			subscription_token TEXT NOT NULL DEFAULT '',
			created_time INTEGER NOT NULL,
			updated_time INTEGER NOT NULL,
			status INTEGER NOT NULL DEFAULT 1
		);`,
		`CREATE TABLE IF NOT EXISTS node (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			secret TEXT NOT NULL UNIQUE,
			ip TEXT NOT NULL,
			server_ip TEXT NOT NULL,
			port_sta INTEGER NOT NULL,
			port_end INTEGER NOT NULL,
			version TEXT NOT NULL DEFAULT '',
			http INTEGER NOT NULL DEFAULT 0,
			tls INTEGER NOT NULL DEFAULT 0,
			socks INTEGER NOT NULL DEFAULT 0,
			created_time INTEGER NOT NULL,
			updated_time INTEGER NOT NULL,
			status INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS tunnel (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			traffic_ratio REAL NOT NULL DEFAULT 1.0,
			in_node_id INTEGER NOT NULL,
			in_ip TEXT NOT NULL,
			out_node_id INTEGER NOT NULL,
			out_ip TEXT NOT NULL,
			type INTEGER NOT NULL,
			protocol TEXT NOT NULL DEFAULT 'tls',
			flow INTEGER NOT NULL DEFAULT 1,
			tcp_listen_addr TEXT NOT NULL DEFAULT '[::]',
			udp_listen_addr TEXT NOT NULL DEFAULT '[::]',
			interface_name TEXT NOT NULL DEFAULT '',
			created_time INTEGER NOT NULL,
			updated_time INTEGER NOT NULL,
			status INTEGER NOT NULL DEFAULT 1
		);`,
		`CREATE TABLE IF NOT EXISTS forward (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			user_name TEXT NOT NULL,
			name TEXT NOT NULL,
			tunnel_id INTEGER NOT NULL,
			in_port INTEGER NOT NULL,
			out_port INTEGER NOT NULL DEFAULT 0,
			remote_addr TEXT NOT NULL,
			strategy TEXT NOT NULL DEFAULT 'fifo',
			interface_name TEXT NOT NULL DEFAULT '',
			in_flow INTEGER NOT NULL DEFAULT 0,
			out_flow INTEGER NOT NULL DEFAULT 0,
			created_time INTEGER NOT NULL,
			updated_time INTEGER NOT NULL,
			status INTEGER NOT NULL DEFAULT 1,
			inx INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS speed_limit (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			speed INTEGER NOT NULL,
			tunnel_id INTEGER NOT NULL,
			tunnel_name TEXT NOT NULL,
			created_time INTEGER NOT NULL,
			updated_time INTEGER NOT NULL,
			status INTEGER NOT NULL DEFAULT 1
		);`,
		`CREATE TABLE IF NOT EXISTS user_tunnel (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			tunnel_id INTEGER NOT NULL,
			speed_id INTEGER,
			num INTEGER NOT NULL,
			flow INTEGER NOT NULL,
			in_flow INTEGER NOT NULL DEFAULT 0,
			out_flow INTEGER NOT NULL DEFAULT 0,
			flow_reset_time INTEGER NOT NULL,
			exp_time INTEGER NOT NULL,
			status INTEGER NOT NULL DEFAULT 1,
			UNIQUE(user_id, tunnel_id)
		);`,
		`CREATE TABLE IF NOT EXISTS statistics_flow (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			flow INTEGER NOT NULL,
			total_flow INTEGER NOT NULL,
			time TEXT NOT NULL,
			created_time INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS vite_config (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			value TEXT NOT NULL,
			time INTEGER NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_forward_user_id ON forward(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_forward_tunnel_id ON forward(tunnel_id);`,
		`CREATE INDEX IF NOT EXISTS idx_user_tunnel_user_id ON user_tunnel(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_user_tunnel_tunnel_id ON user_tunnel(tunnel_id);`,
		`CREATE INDEX IF NOT EXISTS idx_statistics_user_id ON statistics_flow(user_id);`,
	}

	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("schema init failed: %w", err)
		}
	}
	return nil
}

func (s *Server) seedDefaults() error {
	defaults := map[string]string{
		"app_name":        "flux",
		"captcha_enabled": "false",
		"captcha_type":    "RANDOM",
		"ip":              "",
	}
	for key, value := range defaults {
		existing, err := s.getConfigByName(key)
		if err == nil && existing.ID > 0 {
			continue
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err := s.upsertConfig(key, value); err != nil {
			return err
		}
	}

	user, err := s.getUserByUsername(s.cfg.AdminUsername)
	if err == nil && user.ID > 0 {
		return nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	passwordHash, err := hashPassword(s.cfg.AdminPassword)
	if err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	_, err = s.db.Exec(
		`INSERT INTO user (user, pwd, role_id, exp_time, flow, in_flow, out_flow, flow_reset_time, num, subscription_token, created_time, updated_time, status)
		 VALUES (?, ?, 0, ?, 99999, 0, 0, 0, 99999, ?, ?, ?, 1)`,
		s.cfg.AdminUsername,
		passwordHash,
		now+10*365*24*60*60*1000,
		randomToken(48),
		now,
		now,
	)
	if err != nil {
		return err
	}
	log.Printf("bootstrap admin password: %s", s.cfg.AdminPassword)
	return nil
}

func (s *Server) scanUser(scanner interface{ Scan(dest ...interface{}) error }) (User, error) {
	var user User
	var subscription sql.NullString
	err := scanner.Scan(
		&user.ID,
		&user.User,
		&user.Pwd,
		&user.RoleID,
		&user.ExpTime,
		&user.Flow,
		&user.InFlow,
		&user.OutFlow,
		&user.FlowResetTime,
		&user.Num,
		&subscription,
		&user.CreatedTime,
		&user.UpdatedTime,
		&user.Status,
	)
	user.SubscriptionToken = subscription.String
	return user, err
}

func (s *Server) scanNode(scanner interface{ Scan(dest ...interface{}) error }) (Node, error) {
	var node Node
	err := scanner.Scan(
		&node.ID,
		&node.Name,
		&node.Secret,
		&node.IP,
		&node.ServerIP,
		&node.PortSta,
		&node.PortEnd,
		&node.Version,
		&node.HTTP,
		&node.TLS,
		&node.Socks,
		&node.CreatedTime,
		&node.UpdatedTime,
		&node.Status,
	)
	return node, err
}

func (s *Server) scanTunnel(scanner interface{ Scan(dest ...interface{}) error }) (Tunnel, error) {
	var tunnel Tunnel
	var interfaceName sql.NullString
	err := scanner.Scan(
		&tunnel.ID,
		&tunnel.Name,
		&tunnel.TrafficRatio,
		&tunnel.InNodeID,
		&tunnel.InIP,
		&tunnel.OutNodeID,
		&tunnel.OutIP,
		&tunnel.Type,
		&tunnel.Protocol,
		&tunnel.Flow,
		&tunnel.TCPListenAddr,
		&tunnel.UDPListenAddr,
		&interfaceName,
		&tunnel.CreatedTime,
		&tunnel.UpdatedTime,
		&tunnel.Status,
	)
	tunnel.InterfaceName = interfaceName.String
	return tunnel, err
}

func (s *Server) scanForward(scanner interface{ Scan(dest ...interface{}) error }) (Forward, error) {
	var forward Forward
	var interfaceName sql.NullString
	var tunnelName sql.NullString
	var inIP sql.NullString
	var outIP sql.NullString
	var protocol sql.NullString
	var tunnelType sql.NullInt64
	err := scanner.Scan(
		&forward.ID,
		&forward.UserID,
		&forward.UserName,
		&forward.Name,
		&forward.TunnelID,
		&forward.InPort,
		&forward.OutPort,
		&forward.RemoteAddr,
		&forward.Strategy,
		&interfaceName,
		&forward.InFlow,
		&forward.OutFlow,
		&forward.CreatedTime,
		&forward.UpdatedTime,
		&forward.Status,
		&forward.Inx,
		&tunnelName,
		&inIP,
		&outIP,
		&tunnelType,
		&protocol,
	)
	forward.InterfaceName = interfaceName.String
	forward.TunnelName = tunnelName.String
	forward.InIP = inIP.String
	forward.OutIP = outIP.String
	if tunnelType.Valid {
		forward.Type = int(tunnelType.Int64)
	}
	forward.Protocol = protocol.String
	return forward, err
}

func (s *Server) scanUserTunnel(scanner interface{ Scan(dest ...interface{}) error }) (UserTunnel, error) {
	var item UserTunnel
	var speedID sql.NullInt64
	var tunnelName sql.NullString
	var speedLimitName sql.NullString
	var speed sql.NullInt64
	var tunnelFlow sql.NullInt64
	err := scanner.Scan(
		&item.ID,
		&item.UserID,
		&item.TunnelID,
		&item.Flow,
		&item.InFlow,
		&item.OutFlow,
		&item.FlowResetTime,
		&item.ExpTime,
		&speedID,
		&item.Num,
		&item.Status,
		&tunnelName,
		&tunnelFlow,
		&speedLimitName,
		&speed,
	)
	if speedID.Valid {
		value := speedID.Int64
		item.SpeedID = &value
	}
	if speed.Valid {
		intValue := int(speed.Int64)
		item.Speed = &intValue
	}
	item.TunnelName = tunnelName.String
	item.TunnelFlow = int(tunnelFlow.Int64)
	item.SpeedLimitName = speedLimitName.String
	return item, err
}

func (s *Server) scanSpeedLimit(scanner interface{ Scan(dest ...interface{}) error }) (SpeedLimit, error) {
	var item SpeedLimit
	err := scanner.Scan(
		&item.ID,
		&item.Name,
		&item.Speed,
		&item.TunnelID,
		&item.TunnelName,
		&item.CreatedTime,
		&item.UpdatedTime,
		&item.Status,
	)
	return item, err
}

func (s *Server) scanStatistics(scanner interface{ Scan(dest ...interface{}) error }) (StatisticsFlow, error) {
	var item StatisticsFlow
	err := scanner.Scan(
		&item.ID,
		&item.UserID,
		&item.Flow,
		&item.TotalFlow,
		&item.Time,
		&item.CreatedTime,
	)
	return item, err
}

func (s *Server) scanConfig(scanner interface{ Scan(dest ...interface{}) error }) (ViteConfig, error) {
	var cfg ViteConfig
	err := scanner.Scan(&cfg.ID, &cfg.Name, &cfg.Value, &cfg.Time)
	return cfg, err
}

func (s *Server) getUserByUsername(username string) (User, error) {
	row := s.db.QueryRow(`SELECT id, user, pwd, role_id, exp_time, flow, in_flow, out_flow, flow_reset_time, num, subscription_token, created_time, updated_time, status FROM user WHERE user = ? LIMIT 1`, username)
	return s.scanUser(row)
}

func (s *Server) getUserByID(id int64) (User, error) {
	row := s.db.QueryRow(`SELECT id, user, pwd, role_id, exp_time, flow, in_flow, out_flow, flow_reset_time, num, subscription_token, created_time, updated_time, status FROM user WHERE id = ? LIMIT 1`, id)
	return s.scanUser(row)
}

func (s *Server) listUsers(keyword string) ([]User, error) {
	args := []interface{}{}
	query := `SELECT id, user, pwd, role_id, exp_time, flow, in_flow, out_flow, flow_reset_time, num, subscription_token, created_time, updated_time, status FROM user WHERE role_id <> 0`
	if strings.TrimSpace(keyword) != "" {
		query += ` AND user LIKE ?`
		args = append(args, "%"+strings.TrimSpace(keyword)+"%")
	}
	query += ` ORDER BY id DESC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []User
	for rows.Next() {
		item, err := s.scanUser(rows)
		if err != nil {
			return nil, err
		}
		item.Pwd = ""
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Server) getNodeByID(id int64) (Node, error) {
	row := s.db.QueryRow(`SELECT id, name, secret, ip, server_ip, port_sta, port_end, version, http, tls, socks, created_time, updated_time, status FROM node WHERE id = ? LIMIT 1`, id)
	return s.scanNode(row)
}

func (s *Server) getNodeBySecret(secret string) (Node, error) {
	row := s.db.QueryRow(`SELECT id, name, secret, ip, server_ip, port_sta, port_end, version, http, tls, socks, created_time, updated_time, status FROM node WHERE secret = ? LIMIT 1`, secret)
	return s.scanNode(row)
}

func (s *Server) listNodes() ([]Node, error) {
	rows, err := s.db.Query(`SELECT id, name, secret, ip, server_ip, port_sta, port_end, version, http, tls, socks, created_time, updated_time, status FROM node ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Node
	for rows.Next() {
		item, err := s.scanNode(rows)
		if err != nil {
			return nil, err
		}
		item.Secret = ""
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Server) getTunnelByID(id int64) (Tunnel, error) {
	row := s.db.QueryRow(`SELECT id, name, traffic_ratio, in_node_id, in_ip, out_node_id, out_ip, type, protocol, flow, tcp_listen_addr, udp_listen_addr, interface_name, created_time, updated_time, status FROM tunnel WHERE id = ? LIMIT 1`, id)
	return s.scanTunnel(row)
}

func (s *Server) listTunnels() ([]Tunnel, error) {
	rows, err := s.db.Query(`SELECT id, name, traffic_ratio, in_node_id, in_ip, out_node_id, out_ip, type, protocol, flow, tcp_listen_addr, udp_listen_addr, interface_name, created_time, updated_time, status FROM tunnel ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Tunnel
	for rows.Next() {
		item, err := s.scanTunnel(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Server) getForwardByID(id int64) (Forward, error) {
	row := s.db.QueryRow(`
		SELECT f.id, f.user_id, f.user_name, f.name, f.tunnel_id, f.in_port, f.out_port, f.remote_addr, f.strategy, f.interface_name, f.in_flow, f.out_flow, f.created_time, f.updated_time, f.status, f.inx,
		       t.name, t.in_ip, t.out_ip, t.type, t.protocol
		FROM forward f
		LEFT JOIN tunnel t ON t.id = f.tunnel_id
		WHERE f.id = ? LIMIT 1`, id)
	return s.scanForward(row)
}

func (s *Server) listForwards(userID *int64) ([]Forward, error) {
	args := []interface{}{}
	query := `
		SELECT f.id, f.user_id, f.user_name, f.name, f.tunnel_id, f.in_port, f.out_port, f.remote_addr, f.strategy, f.interface_name, f.in_flow, f.out_flow, f.created_time, f.updated_time, f.status, f.inx,
		       t.name, t.in_ip, t.out_ip, t.type, t.protocol
		FROM forward f
		LEFT JOIN tunnel t ON t.id = f.tunnel_id`
	if userID != nil {
		query += ` WHERE f.user_id = ?`
		args = append(args, *userID)
	}
	query += ` ORDER BY f.created_time DESC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Forward
	for rows.Next() {
		item, err := s.scanForward(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Server) listForwardsByUser(userID int64) ([]Forward, error) {
	return s.listForwards(&userID)
}

func (s *Server) listForwardsByUserAndTunnel(userID, tunnelID int64) ([]Forward, error) {
	rows, err := s.db.Query(`
		SELECT f.id, f.user_id, f.user_name, f.name, f.tunnel_id, f.in_port, f.out_port, f.remote_addr, f.strategy, f.interface_name, f.in_flow, f.out_flow, f.created_time, f.updated_time, f.status, f.inx,
		       t.name, t.in_ip, t.out_ip, t.type, t.protocol
		FROM forward f
		LEFT JOIN tunnel t ON t.id = f.tunnel_id
		WHERE f.user_id = ? AND f.tunnel_id = ?
		ORDER BY f.created_time DESC`, userID, tunnelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Forward
	for rows.Next() {
		item, err := s.scanForward(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Server) listForwardsByTunnel(tunnelID int64) ([]Forward, error) {
	rows, err := s.db.Query(`
		SELECT f.id, f.user_id, f.user_name, f.name, f.tunnel_id, f.in_port, f.out_port, f.remote_addr, f.strategy, f.interface_name, f.in_flow, f.out_flow, f.created_time, f.updated_time, f.status, f.inx,
		       t.name, t.in_ip, t.out_ip, t.type, t.protocol
		FROM forward f
		LEFT JOIN tunnel t ON t.id = f.tunnel_id
		WHERE f.tunnel_id = ?
		ORDER BY f.created_time DESC`, tunnelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Forward
	for rows.Next() {
		item, err := s.scanForward(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Server) getUserTunnelByID(id int64) (UserTunnel, error) {
	row := s.db.QueryRow(`
		SELECT ut.id, ut.user_id, ut.tunnel_id, ut.flow, ut.in_flow, ut.out_flow, ut.flow_reset_time, ut.exp_time, ut.speed_id, ut.num, ut.status,
		       t.name, t.flow, sl.name, sl.speed
		FROM user_tunnel ut
		LEFT JOIN tunnel t ON t.id = ut.tunnel_id
		LEFT JOIN speed_limit sl ON sl.id = ut.speed_id
		WHERE ut.id = ? LIMIT 1`, id)
	return s.scanUserTunnel(row)
}

func (s *Server) getUserTunnelByUserAndTunnel(userID, tunnelID int64) (UserTunnel, error) {
	row := s.db.QueryRow(`
		SELECT ut.id, ut.user_id, ut.tunnel_id, ut.flow, ut.in_flow, ut.out_flow, ut.flow_reset_time, ut.exp_time, ut.speed_id, ut.num, ut.status,
		       t.name, t.flow, sl.name, sl.speed
		FROM user_tunnel ut
		LEFT JOIN tunnel t ON t.id = ut.tunnel_id
		LEFT JOIN speed_limit sl ON sl.id = ut.speed_id
		WHERE ut.user_id = ? AND ut.tunnel_id = ? LIMIT 1`, userID, tunnelID)
	return s.scanUserTunnel(row)
}

func (s *Server) listUserTunnels(userID int64) ([]UserTunnel, error) {
	rows, err := s.db.Query(`
		SELECT ut.id, ut.user_id, ut.tunnel_id, ut.flow, ut.in_flow, ut.out_flow, ut.flow_reset_time, ut.exp_time, ut.speed_id, ut.num, ut.status,
		       t.name, t.flow, sl.name, sl.speed
		FROM user_tunnel ut
		LEFT JOIN tunnel t ON t.id = ut.tunnel_id
		LEFT JOIN speed_limit sl ON sl.id = ut.speed_id
		WHERE ut.user_id = ?
		ORDER BY ut.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []UserTunnel
	for rows.Next() {
		item, err := s.scanUserTunnel(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Server) listAccessibleTunnels(userID int64, roleID int) ([]Tunnel, error) {
	if roleID == 0 {
		rows, err := s.db.Query(`
			SELECT t.id, t.name, t.traffic_ratio, t.in_node_id, t.in_ip, t.out_node_id, t.out_ip, t.type, t.protocol, t.flow, t.tcp_listen_addr, t.udp_listen_addr, t.interface_name, t.created_time, t.updated_time, t.status
			FROM tunnel t
			WHERE t.status = 1
			ORDER BY t.id`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var items []Tunnel
		for rows.Next() {
			item, err := s.scanTunnel(rows)
			if err != nil {
				return nil, err
			}
			if node, err := s.getNodeByID(item.InNodeID); err == nil {
				item.InNodePortSta = node.PortSta
				item.InNodePortEnd = node.PortEnd
			}
			items = append(items, item)
		}
		return items, rows.Err()
	}

	rows, err := s.db.Query(`
		SELECT t.id, t.name, t.traffic_ratio, t.in_node_id, t.in_ip, t.out_node_id, t.out_ip, t.type, t.protocol, t.flow, t.tcp_listen_addr, t.udp_listen_addr, t.interface_name, t.created_time, t.updated_time, t.status
		FROM tunnel t
		INNER JOIN user_tunnel ut ON ut.tunnel_id = t.id
		WHERE ut.user_id = ? AND ut.status = 1 AND t.status = 1
		ORDER BY t.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Tunnel
	for rows.Next() {
		item, err := s.scanTunnel(rows)
		if err != nil {
			return nil, err
		}
		if node, err := s.getNodeByID(item.InNodeID); err == nil {
			item.InNodePortSta = node.PortSta
			item.InNodePortEnd = node.PortEnd
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Server) getSpeedLimitByID(id int64) (SpeedLimit, error) {
	row := s.db.QueryRow(`SELECT id, name, speed, tunnel_id, tunnel_name, created_time, updated_time, status FROM speed_limit WHERE id = ? LIMIT 1`, id)
	return s.scanSpeedLimit(row)
}

func (s *Server) listSpeedLimits() ([]SpeedLimit, error) {
	rows, err := s.db.Query(`SELECT id, name, speed, tunnel_id, tunnel_name, created_time, updated_time, status FROM speed_limit ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []SpeedLimit
	for rows.Next() {
		item, err := s.scanSpeedLimit(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Server) getConfigByName(name string) (ViteConfig, error) {
	row := s.db.QueryRow(`SELECT id, name, value, time FROM vite_config WHERE name = ? LIMIT 1`, name)
	return s.scanConfig(row)
}

func (s *Server) getConfigs() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT id, name, value, time FROM vite_config ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		item, err := s.scanConfig(rows)
		if err != nil {
			return nil, err
		}
		out[item.Name] = item.Value
	}
	return out, rows.Err()
}

func (s *Server) upsertConfig(name, value string) error {
	now := time.Now().UnixMilli()
	_, err := s.db.Exec(`
		INSERT INTO vite_config (name, value, time)
		VALUES (?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET value = excluded.value, time = excluded.time`, name, value, now)
	return err
}

func (s *Server) listStatisticsForUser(userID int64, limit int) ([]StatisticsFlow, error) {
	rows, err := s.db.Query(`SELECT id, user_id, flow, total_flow, time, created_time FROM statistics_flow WHERE user_id = ? ORDER BY id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []StatisticsFlow
	for rows.Next() {
		item, err := s.scanStatistics(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Server) insertStatisticsFlows(items []StatisticsFlow) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.Prepare(`INSERT INTO statistics_flow (user_id, flow, total_flow, time, created_time) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, item := range items {
		if _, err := stmt.Exec(item.UserID, item.Flow, item.TotalFlow, item.Time, item.CreatedTime); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Server) deleteExpiredStatistics(cutoff int64) error {
	_, err := s.db.Exec(`DELETE FROM statistics_flow WHERE created_time < ?`, cutoff)
	return err
}
