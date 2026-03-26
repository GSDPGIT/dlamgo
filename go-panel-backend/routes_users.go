package main

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (s *Server) handleUserLogin(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if !s.loginLimiter.Allow(clientIP(r)) {
		writeJSON(w, http.StatusTooManyRequests, errResp(-1, "登录过于频繁，请稍后再试"))
		return
	}

	var req LoginRequest
	if err := s.decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(-1, "请求参数错误"))
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, errResp(-1, "用户名或密码不能为空"))
		return
	}

	if captchaEnabled, _ := s.getConfigValueBool("captcha_enabled"); captchaEnabled {
		if !s.captcha.Verify(strings.TrimSpace(req.CaptchaID)) {
			writeJSON(w, http.StatusOK, errResp(-1, "验证码校验失败"))
			return
		}
	}

	user, err := s.getUserByUsername(req.Username)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "账号或密码错误"))
		return
	}
	if !checkPassword(user.Pwd, req.Password) {
		writeJSON(w, http.StatusOK, errResp(-1, "账号或密码错误"))
		return
	}
	if user.Status == 0 {
		writeJSON(w, http.StatusOK, errResp(-1, "账户停用"))
		return
	}
	if user.ExpTime > 0 && user.ExpTime <= time.Now().UnixMilli() {
		writeJSON(w, http.StatusOK, errResp(-1, "账户已到期"))
		return
	}

	token, err := generateToken(s.cfg, user)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(-2, "JWT token generation failed"))
		return
	}

	writeJSON(w, http.StatusOK, ok(LoginPayload{
		Token:                 token,
		RoleID:                user.RoleID,
		Name:                  user.User,
		RequirePasswordChange: s.requiresPasswordChange(req.Username, req.Password),
	}))
}

func (s *Server) handleUserCreate(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if _, err := s.requireAdmin(r); err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	var req UserDTO
	if err := s.decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(-1, "请求参数错误"))
		return
	}
	req.User = strings.TrimSpace(req.User)
	if req.User == "" || strings.TrimSpace(req.Pwd) == "" {
		writeJSON(w, http.StatusOK, errResp(-1, "用户名和密码不能为空"))
		return
	}
	if _, err := s.getUserByUsername(req.User); err == nil {
		writeJSON(w, http.StatusOK, errResp(-1, "用户名已存在"))
		return
	}
	passwordHash, err := hashPassword(req.Pwd)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(-2, "密码加密失败"))
		return
	}
	status := 1
	if req.Status != nil {
		status = *req.Status
	}
	now := time.Now().UnixMilli()
	_, err = s.db.Exec(`
		INSERT INTO user (user, pwd, role_id, exp_time, flow, in_flow, out_flow, flow_reset_time, num, subscription_token, created_time, updated_time, status)
		VALUES (?, ?, 1, ?, ?, 0, 0, ?, ?, ?, ?, ?, ?)`,
		req.User, passwordHash, req.ExpTime, req.Flow, req.FlowResetTime, req.Num, randomToken(48), now, now, status,
	)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "用户创建失败"))
		return
	}
	writeJSON(w, http.StatusOK, ok("用户创建成功"))
}

func (s *Server) handleUserList(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if _, err := s.requireAdmin(r); err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	var params map[string]interface{}
	_ = s.decodeJSON(r, &params)
	keyword := ""
	if value, ok := params["keyword"].(string); ok {
		keyword = value
	}
	users, err := s.listUsers(keyword)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(-2, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, ok(users))
}

func (s *Server) handleUserUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if _, err := s.requireAdmin(r); err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	var req UserUpdateDTO
	if err := s.decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(-1, "请求参数错误"))
		return
	}
	existing, err := s.getUserByID(req.ID)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "用户不存在"))
		return
	}
	if existing.RoleID == 0 {
		writeJSON(w, http.StatusOK, errResp(-1, "不能修改管理员用户信息"))
		return
	}
	req.User = strings.TrimSpace(req.User)
	if req.User == "" {
		writeJSON(w, http.StatusOK, errResp(-1, "用户名不能为空"))
		return
	}
	if other, err := s.getUserByUsername(req.User); err == nil && other.ID != req.ID {
		writeJSON(w, http.StatusOK, errResp(-1, "用户名已被其他用户使用"))
		return
	}

	passwordHash := existing.Pwd
	if req.Pwd != nil && strings.TrimSpace(*req.Pwd) != "" {
		passwordHash, err = hashPassword(strings.TrimSpace(*req.Pwd))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errResp(-2, "密码加密失败"))
			return
		}
	}
	status := existing.Status
	if req.Status != nil {
		status = *req.Status
	}
	_, err = s.db.Exec(`
		UPDATE user
		SET user = ?, pwd = ?, exp_time = ?, flow = ?, flow_reset_time = ?, num = ?, updated_time = ?, status = ?
		WHERE id = ?`,
		req.User, passwordHash, req.ExpTime, req.Flow, req.FlowResetTime, req.Num, time.Now().UnixMilli(), status, req.ID,
	)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "用户更新失败"))
		return
	}
	writeJSON(w, http.StatusOK, ok("用户更新成功"))
}

func (s *Server) handleUserDelete(w http.ResponseWriter, r *http.Request) {
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
	user, err := s.getUserByID(id)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "用户不存在"))
		return
	}
	if user.RoleID == 0 {
		writeJSON(w, http.StatusOK, errResp(-1, "不能删除管理员用户"))
		return
	}

	forwards, _ := s.listForwardsByUser(id)
	for _, forward := range forwards {
		_ = s.cleanupForwardServices(forward)
	}

	tx, err := s.db.Begin()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(-2, err.Error()))
		return
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM forward WHERE user_id = ?`, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(-2, err.Error()))
		return
	}
	if _, err := tx.Exec(`DELETE FROM user_tunnel WHERE user_id = ?`, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(-2, err.Error()))
		return
	}
	if _, err := tx.Exec(`DELETE FROM statistics_flow WHERE user_id = ?`, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(-2, err.Error()))
		return
	}
	if _, err := tx.Exec(`DELETE FROM user WHERE id = ?`, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(-2, err.Error()))
		return
	}
	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(-2, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, ok("用户及关联数据删除成功"))
}

func (s *Server) handleUserPackage(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	claims, err := s.requireAuth(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	userID, _ := strconv.ParseInt(claims.Sub, 10, 64)
	user, err := s.getUserByID(userID)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "用户不存在"))
		return
	}
	tunnels, _ := s.listUserTunnels(userID)
	forwards, _ := s.listForwardsByUser(userID)
	stats, _ := s.listStatisticsForUser(userID, 24)

	writeJSON(w, http.StatusOK, ok(UserPackageResponse{
		UserInfo: UserPackageUserInfo{
			ID:                user.ID,
			Name:              user.Name,
			User:              user.User,
			Status:            user.Status,
			Flow:              user.Flow,
			InFlow:            user.InFlow,
			OutFlow:           user.OutFlow,
			Num:               user.Num,
			ExpTime:           user.ExpTime,
			FlowResetTime:     user.FlowResetTime,
			CreatedTime:       user.CreatedTime,
			UpdatedTime:       user.UpdatedTime,
			SubscriptionToken: user.SubscriptionToken,
		},
		TunnelPermissions: tunnels,
		Forwards:          forwards,
		StatisticsFlows:   normalizeStatistics(stats),
	}))
}

func (s *Server) handleUserPasswordUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	claims, err := s.requireAuth(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	var req ChangePasswordDTO
	if err := s.decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(-1, "请求参数错误"))
		return
	}
	if req.NewPassword != req.ConfirmPassword {
		writeJSON(w, http.StatusOK, errResp(-1, "新密码和确认密码不匹配"))
		return
	}
	userID, _ := strconv.ParseInt(claims.Sub, 10, 64)
	user, err := s.getUserByID(userID)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "用户不存在"))
		return
	}
	if !checkPassword(user.Pwd, req.CurrentPassword) {
		writeJSON(w, http.StatusOK, errResp(-1, "当前密码错误"))
		return
	}
	req.NewUsername = strings.TrimSpace(req.NewUsername)
	if req.NewUsername == "" {
		writeJSON(w, http.StatusOK, errResp(-1, "新用户名不能为空"))
		return
	}
	if other, err := s.getUserByUsername(req.NewUsername); err == nil && other.ID != userID {
		writeJSON(w, http.StatusOK, errResp(-1, "用户名已被其他用户使用"))
		return
	}
	hash, err := hashPassword(req.NewPassword)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(-2, "密码加密失败"))
		return
	}
	_, err = s.db.Exec(`UPDATE user SET user = ?, pwd = ?, updated_time = ? WHERE id = ?`, req.NewUsername, hash, time.Now().UnixMilli(), userID)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "用户更新失败"))
		return
	}
	writeJSON(w, http.StatusOK, ok("账号密码修改成功"))
}

func (s *Server) handleResetFlow(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if _, err := s.requireAdmin(r); err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	var req ResetFlowDTO
	if err := s.decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(-1, "请求参数错误"))
		return
	}
	switch req.Type {
	case 1:
		if _, err := s.db.Exec(`UPDATE user SET in_flow = 0, out_flow = 0, updated_time = ? WHERE id = ?`, time.Now().UnixMilli(), req.ID); err != nil {
			writeJSON(w, http.StatusInternalServerError, errResp(-2, err.Error()))
			return
		}
	case 2:
		if _, err := s.db.Exec(`UPDATE user_tunnel SET in_flow = 0, out_flow = 0 WHERE id = ?`, req.ID); err != nil {
			writeJSON(w, http.StatusInternalServerError, errResp(-2, err.Error()))
			return
		}
	default:
		writeJSON(w, http.StatusOK, errResp(-1, "重置类型无效"))
		return
	}
	writeJSON(w, http.StatusOK, okNoData())
}

func (s *Server) requiresPasswordChange(username, password string) bool {
	return username == "admin_user" && password == "admin_user"
}

func (s *Server) getConfigValueBool(name string) (bool, error) {
	cfg, err := s.getConfigByName(name)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(cfg.Value, "true"), nil
}

func normalizeStatistics(items []StatisticsFlow) []StatisticsFlow {
	byHour := map[string]StatisticsFlow{}
	for _, item := range items {
		byHour[item.Time] = item
	}
	now := time.Now()
	normalized := make([]StatisticsFlow, 0, 24)
	for i := 23; i >= 0; i-- {
		hour := now.Add(-time.Duration(i) * time.Hour).Format("15:00")
		if item, ok := byHour[hour]; ok {
			normalized = append(normalized, item)
			continue
		}
		normalized = append(normalized, StatisticsFlow{
			UserID:    0,
			Flow:      0,
			TotalFlow: 0,
			Time:      hour,
		})
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		return normalized[i].Time < normalized[j].Time
	})
	return normalized
}

func mapInt64(value interface{}) int64 {
	switch v := value.(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	case string:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return parsed
	default:
		return 0
	}
}

func (s *Server) cleanupForwardServices(forward Forward) error {
	tunnel, err := s.getTunnelByID(forward.TunnelID)
	if err != nil {
		return err
	}
	userTunnel, _ := s.getUserTunnelByUserAndTunnel(forward.UserID, forward.TunnelID)
	serviceName := buildServiceName(forward.ID, forward.UserID, userTunnel.ID)
	_ = s.sendNodeCommand(tunnel.InNodeID, "DeleteService", deleteServicesPayload(serviceName+"_tcp", serviceName+"_udp"))
	if tunnel.Type == 2 {
		_ = s.sendNodeCommand(tunnel.InNodeID, "DeleteChains", deleteChainPayload(serviceName+"_chains"))
		_ = s.sendNodeCommand(tunnel.OutNodeID, "DeleteService", deleteServicesPayload(serviceName+"_tls"))
	}
	return nil
}

func buildServiceName(forwardID, userID, userTunnelID int64) string {
	return strconv.FormatInt(forwardID, 10) + "_" + strconv.FormatInt(userID, 10) + "_" + strconv.FormatInt(userTunnelID, 10)
}
