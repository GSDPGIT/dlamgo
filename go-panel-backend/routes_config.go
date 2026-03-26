package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

var publicConfigKeys = map[string]struct{}{
	"app_name":        {},
	"captcha_enabled": {},
	"captcha_type":    {},
}

func (s *Server) handleConfigList(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	configs, err := s.getConfigs()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(-2, err.Error()))
		return
	}
	if claims, err := s.requireAuth(r); err == nil && claims.RoleID == 0 {
		writeJSON(w, http.StatusOK, ok(configs))
		return
	}
	public := map[string]string{}
	for key, value := range configs {
		if _, ok := publicConfigKeys[key]; ok {
			public[key] = value
		}
	}
	writeJSON(w, http.StatusOK, ok(public))
}

func (s *Server) handleConfigGet(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var params map[string]interface{}
	if err := s.decodeJSON(r, &params); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(-1, "请求参数错误"))
		return
	}
	name, _ := params["name"].(string)
	name = strings.TrimSpace(name)
	if name == "" {
		writeJSON(w, http.StatusOK, errResp(-1, "配置名称不能为空"))
		return
	}
	if _, ok := publicConfigKeys[name]; !ok {
		if claims, err := s.requireAuth(r); err != nil || claims.RoleID != 0 {
			writeJSON(w, http.StatusUnauthorized, errResp(401, "权限不足"))
			return
		}
	}
	cfg, err := s.getConfigByName(name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusOK, errResp(-1, "配置不存在"))
			return
		}
		writeJSON(w, http.StatusInternalServerError, errResp(-2, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, ok(cfg))
}

func (s *Server) handleConfigUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if _, err := s.requireAdmin(r); err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp(401, err.Error()))
		return
	}
	var configMap map[string]string
	if err := s.decodeJSON(r, &configMap); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(-1, "请求参数错误"))
		return
	}
	if len(configMap) == 0 {
		writeJSON(w, http.StatusOK, errResp(-1, "配置数据不能为空"))
		return
	}
	for key, value := range configMap {
		if strings.TrimSpace(key) == "" {
			continue
		}
		if err := s.upsertConfig(strings.TrimSpace(key), value); err != nil {
			writeJSON(w, http.StatusInternalServerError, errResp(-2, err.Error()))
			return
		}
	}
	writeJSON(w, http.StatusOK, ok("配置更新成功"))
}

func (s *Server) handleConfigUpdateSingle(w http.ResponseWriter, r *http.Request) {
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
	name, _ := params["name"].(string)
	value, _ := params["value"].(string)
	name = strings.TrimSpace(name)
	if name == "" || strings.TrimSpace(value) == "" {
		writeJSON(w, http.StatusOK, errResp(-1, "配置名称和值不能为空"))
		return
	}
	if err := s.upsertConfig(name, value); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(-2, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, ok("配置更新成功"))
}

func (s *Server) handleCaptchaCheck(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	enabled, _ := s.getConfigValueBool("captcha_enabled")
	if enabled {
		writeJSON(w, http.StatusOK, ok(1))
		return
	}
	writeJSON(w, http.StatusOK, ok(0))
}

func (s *Server) handleCaptchaGenerate(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	enabled, _ := s.getConfigValueBool("captcha_enabled")
	if !enabled {
		writeJSON(w, http.StatusOK, errResp(-1, "验证码未启用"))
		return
	}
	challenge := s.captcha.NewChallenge()
	response := CaptchaGenerateResponse{
		ID: challenge.ID,
		Captcha: map[string]interface{}{
			"type":            "SLIDER",
			"backgroundImage": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4////fwAJ+wP+J4iigAAAAABJRU5ErkJggg==",
			"templateImage":   "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4////fwAJ+wP+J4iigAAAAABJRU5ErkJggg==",
		},
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(response)
}

func (s *Server) handleCaptchaVerify(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var req CaptchaVerifyRequest
	if err := s.decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(-1, "请求参数错误"))
		return
	}
	if !s.captcha.Verify(strings.TrimSpace(req.ID)) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"code":    400,
			"msg":     "验证码校验失败",
		})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"code":    200,
		"msg":     "OK",
		"data": map[string]string{
			"validToken": req.ID,
		},
		"ts": time.Now().UnixMilli(),
	})
}
