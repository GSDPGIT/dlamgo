package main

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (s *Server) handleOpenAPISubStore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errResp(-1, "method not allowed"))
		return
	}
	username := strings.TrimSpace(r.URL.Query().Get("user"))
	secret := strings.TrimSpace(r.URL.Query().Get("pwd"))
	tunnelValue := strings.TrimSpace(r.URL.Query().Get("tunnel"))
	if username == "" || secret == "" {
		writeJSON(w, http.StatusOK, errResp(-1, "鉴权失败"))
		return
	}

	user, err := s.getUserByUsername(username)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp(-1, "鉴权失败"))
		return
	}
	if user.SubscriptionToken == "" || !strings.EqualFold(user.SubscriptionToken, secret) {
		writeJSON(w, http.StatusOK, errResp(-1, "鉴权失败"))
		return
	}

	upload := user.OutFlow
	download := user.InFlow
	total := user.Flow * 1024 * 1024 * 1024
	expire := user.ExpTime / 1000

	if tunnelValue != "" && tunnelValue != "-1" {
		userTunnel, err := s.getUserTunnelByID(mapInt64(tunnelValue))
		if err != nil || userTunnel.UserID != user.ID {
			writeJSON(w, http.StatusOK, errResp(-1, "隧道不存在"))
			return
		}
		upload = userTunnel.OutFlow
		download = userTunnel.InFlow
		total = userTunnel.Flow * 1024 * 1024 * 1024
		expire = userTunnel.ExpTime / 1000
	}

	headerValue := buildSubscriptionHeader(upload, download, total, expire)
	w.Header().Set("subscription-userinfo", headerValue)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(headerValue))
}

func buildSubscriptionHeader(upload, download, total, expire int64) string {
	return "upload=" + strconv.FormatInt(upload, 10) +
		"; download=" + strconv.FormatInt(download, 10) +
		"; total=" + strconv.FormatInt(total, 10) +
		"; expire=" + strconv.FormatInt(expire, 10)
}

func (s *Server) getPublicSubscriptionToken(userID int64) (string, error) {
	user, err := s.getUserByID(userID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(user.SubscriptionToken) != "" {
		return user.SubscriptionToken, nil
	}
	token := randomToken(48)
	_, err = s.db.Exec(`UPDATE user SET subscription_token = ?, updated_time = ? WHERE id = ?`, token, time.Now().UnixMilli(), userID)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *Server) findUserTunnelOrZero(userID, tunnelID int64) (UserTunnel, error) {
	item, err := s.getUserTunnelByUserAndTunnel(userID, tunnelID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return UserTunnel{}, nil
		}
		return UserTunnel{}, err
	}
	return item, nil
}
