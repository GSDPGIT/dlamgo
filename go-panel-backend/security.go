package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type TokenClaims struct {
	Sub    string `json:"sub"`
	Iat    int64  `json:"iat"`
	Exp    int64  `json:"exp"`
	User   string `json:"user"`
	Name   string `json:"name"`
	RoleID int    `json:"role_id"`
}

func hashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func checkPassword(hashed, password string) bool {
	if hashed == "" || password == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password)) == nil
}

func generateToken(cfg Config, user User) (string, error) {
	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}
	now := time.Now()
	claims := TokenClaims{
		Sub:    fmt.Sprintf("%d", user.ID),
		Iat:    now.Unix(),
		Exp:    now.Add(cfg.TokenTTL).Unix(),
		User:   user.User,
		Name:   user.User,
		RoleID: user.RoleID,
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	part1 := base64.RawURLEncoding.EncodeToString(headerJSON)
	part2 := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signature := signJWT(cfg.JWTSecret, part1+"."+part2)
	return part1 + "." + part2 + "." + signature, nil
}

func parseToken(cfg Config, token string) (TokenClaims, error) {
	var claims TokenClaims
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return claims, errors.New("invalid token format")
	}
	expected := signJWT(cfg.JWTSecret, parts[0]+"."+parts[1])
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return claims, errors.New("invalid token signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims, err
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return claims, err
	}
	if claims.Exp <= time.Now().Unix() {
		return claims, errors.New("token expired")
	}
	return claims, nil
}

func signJWT(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func encryptSecretPayload(secret string, plaintext []byte) (string, error) {
	return encryptAESGCM(secret, plaintext)
}

func decryptSecretPayload(secret, ciphertext string) ([]byte, error) {
	return decryptAESGCM(secret, ciphertext)
}

func encryptAESGCM(secret string, plaintext []byte) (string, error) {
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	out := append(nonce, ciphertext...)
	return base64.StdEncoding.EncodeToString(out), nil
}

func decryptAESGCM(secret string, ciphertext string) ([]byte, error) {
	key := sha256.Sum256([]byte(secret))
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	nonce := raw[:gcm.NonceSize()]
	payload := raw[gcm.NonceSize():]
	return gcm.Open(nil, nonce, payload, nil)
}

type rateWindow struct {
	Count     int
	StartedAt time.Time
}

type IPRateLimiter struct {
	mu      sync.Mutex
	limit   int
	windows map[string]rateWindow
}

func NewIPRateLimiter(limit int) *IPRateLimiter {
	return &IPRateLimiter{
		limit:   limit,
		windows: map[string]rateWindow{},
	}
}

func (l *IPRateLimiter) Allow(key string) bool {
	if l == nil || l.limit <= 0 {
		return true
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	window := l.windows[key]
	if window.StartedAt.IsZero() || now.Sub(window.StartedAt) >= time.Minute {
		window = rateWindow{Count: 0, StartedAt: now}
	}
	window.Count++
	l.windows[key] = window
	return window.Count <= l.limit
}
