package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type Server struct {
	cfg          Config
	db           *sql.DB
	mux          *http.ServeMux
	loginLimiter *IPRateLimiter
	captcha      *CaptchaStore
	nodeHub      *NodeHub
	ticketStore  *TicketStore
}

func NewServer(cfg Config) (*Server, error) {
	db, err := sql.Open("sqlite", cfg.DatabasePath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	server := &Server{
		cfg:          cfg,
		db:           db,
		mux:          http.NewServeMux(),
		loginLimiter: NewIPRateLimiter(cfg.LoginRateLimit),
		captcha:      NewCaptchaStore(),
		ticketStore:  NewTicketStore(cfg.WSTicketTTL),
	}
	server.nodeHub = NewNodeHub(server)

	if err := server.initSchema(); err != nil {
		return nil, err
	}
	if err := server.seedDefaults(); err != nil {
		return nil, err
	}

	server.registerRoutes()
	server.startBackgroundJobs()
	return server, nil
}

func (s *Server) Start() error {
	return http.ListenAndServe(s.cfg.Addr, s.withMiddleware(s.mux))
}

func (s *Server) withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-site")
		s.applyCORS(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) applyCORS(w http.ResponseWriter, r *http.Request) {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return
	}
	if len(s.cfg.AllowedOrigins) == 0 {
		if strings.HasPrefix(origin, "http://localhost") || strings.HasPrefix(origin, "http://127.0.0.1") {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
	} else {
		for _, allowed := range s.cfg.AllowedOrigins {
			if allowed == origin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				break
			}
		}
	}
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Node-Secret")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Expose-Headers", "Authorization")
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/api/v1/user/login", s.handleUserLogin)
	s.mux.HandleFunc("/api/v1/user/create", s.handleUserCreate)
	s.mux.HandleFunc("/api/v1/user/list", s.handleUserList)
	s.mux.HandleFunc("/api/v1/user/update", s.handleUserUpdate)
	s.mux.HandleFunc("/api/v1/user/delete", s.handleUserDelete)
	s.mux.HandleFunc("/api/v1/user/package", s.handleUserPackage)
	s.mux.HandleFunc("/api/v1/user/updatePassword", s.handleUserPasswordUpdate)
	s.mux.HandleFunc("/api/v1/user/reset", s.handleResetFlow)

	s.mux.HandleFunc("/api/v1/node/create", s.handleNodeCreate)
	s.mux.HandleFunc("/api/v1/node/list", s.handleNodeList)
	s.mux.HandleFunc("/api/v1/node/update", s.handleNodeUpdate)
	s.mux.HandleFunc("/api/v1/node/delete", s.handleNodeDelete)
	s.mux.HandleFunc("/api/v1/node/install", s.handleNodeInstallCommand)
	s.mux.HandleFunc("/api/v1/node/check-status", s.handleNodeCheckStatus)
	s.mux.HandleFunc("/api/v1/node/ws-ticket", s.handleNodeWSTicket)

	s.mux.HandleFunc("/api/v1/tunnel/create", s.handleTunnelCreate)
	s.mux.HandleFunc("/api/v1/tunnel/list", s.handleTunnelList)
	s.mux.HandleFunc("/api/v1/tunnel/get", s.handleTunnelGet)
	s.mux.HandleFunc("/api/v1/tunnel/update", s.handleTunnelUpdate)
	s.mux.HandleFunc("/api/v1/tunnel/delete", s.handleTunnelDelete)
	s.mux.HandleFunc("/api/v1/tunnel/user/assign", s.handleUserTunnelAssign)
	s.mux.HandleFunc("/api/v1/tunnel/user/list", s.handleUserTunnelList)
	s.mux.HandleFunc("/api/v1/tunnel/user/remove", s.handleUserTunnelRemove)
	s.mux.HandleFunc("/api/v1/tunnel/user/update", s.handleUserTunnelUpdate)
	s.mux.HandleFunc("/api/v1/tunnel/user/tunnel", s.handleUserAccessibleTunnels)
	s.mux.HandleFunc("/api/v1/tunnel/diagnose", s.handleTunnelDiagnose)

	s.mux.HandleFunc("/api/v1/forward/create", s.handleForwardCreate)
	s.mux.HandleFunc("/api/v1/forward/list", s.handleForwardList)
	s.mux.HandleFunc("/api/v1/forward/update", s.handleForwardUpdate)
	s.mux.HandleFunc("/api/v1/forward/delete", s.handleForwardDelete)
	s.mux.HandleFunc("/api/v1/forward/force-delete", s.handleForwardForceDelete)
	s.mux.HandleFunc("/api/v1/forward/pause", s.handleForwardPause)
	s.mux.HandleFunc("/api/v1/forward/resume", s.handleForwardResume)
	s.mux.HandleFunc("/api/v1/forward/diagnose", s.handleForwardDiagnose)
	s.mux.HandleFunc("/api/v1/forward/update-order", s.handleForwardUpdateOrder)

	s.mux.HandleFunc("/api/v1/speed-limit/create", s.handleSpeedLimitCreate)
	s.mux.HandleFunc("/api/v1/speed-limit/list", s.handleSpeedLimitList)
	s.mux.HandleFunc("/api/v1/speed-limit/update", s.handleSpeedLimitUpdate)
	s.mux.HandleFunc("/api/v1/speed-limit/delete", s.handleSpeedLimitDelete)
	s.mux.HandleFunc("/api/v1/speed-limit/tunnels", s.handleSpeedLimitTunnels)

	s.mux.HandleFunc("/api/v1/config/list", s.handleConfigList)
	s.mux.HandleFunc("/api/v1/config/get", s.handleConfigGet)
	s.mux.HandleFunc("/api/v1/config/update", s.handleConfigUpdate)
	s.mux.HandleFunc("/api/v1/config/update-single", s.handleConfigUpdateSingle)

	s.mux.HandleFunc("/api/v1/captcha/check", s.handleCaptchaCheck)
	s.mux.HandleFunc("/api/v1/captcha/generate", s.handleCaptchaGenerate)
	s.mux.HandleFunc("/api/v1/captcha/verify", s.handleCaptchaVerify)

	s.mux.HandleFunc("/api/v1/open_api/sub_store", s.handleOpenAPISubStore)

	s.mux.HandleFunc("/flow/config", s.handleFlowConfig)
	s.mux.HandleFunc("/flow/upload", s.handleFlowUpload)
	s.mux.HandleFunc("/flow/test", s.handleFlowTest)
	s.mux.HandleFunc("/system-info", s.handleSystemInfoWebSocket)
}

func (s *Server) decodeJSON(r *http.Request, dst interface{}) error {
	defer r.Body.Close()
	data, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return errors.New("empty request body")
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return err
	}
	return nil
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		writeJSON(w, http.StatusMethodNotAllowed, errResp(-1, "method not allowed"))
		return false
	}
	return true
}

func (s *Server) requireAuth(r *http.Request) (TokenClaims, error) {
	token := strings.TrimSpace(r.Header.Get("Authorization"))
	if token == "" {
		return TokenClaims{}, errors.New("未登录或token已过期")
	}
	return parseToken(s.cfg, token)
}

func (s *Server) requireAdmin(r *http.Request) (TokenClaims, error) {
	claims, err := s.requireAuth(r)
	if err != nil {
		return TokenClaims{}, err
	}
	if claims.RoleID != 0 {
		return TokenClaims{}, errors.New("权限不足，仅管理员可操作")
	}
	return claims, nil
}

func (s *Server) startBackgroundJobs() {
	go s.runHourlyStatsJob()
	go s.runDailyResetJob()
}

func clientIP(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if rip := strings.TrimSpace(r.Header.Get("X-Real-IP")); rip != "" {
		return rip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

type TicketStore struct {
	mu    sync.Mutex
	ttl   time.Duration
	items map[string]Ticket
}

func NewTicketStore(ttl time.Duration) *TicketStore {
	return &TicketStore{
		ttl:   ttl,
		items: map[string]Ticket{},
	}
}

func (s *TicketStore) New(userID int64, roleID int) Ticket {
	ticket := Ticket{
		Value:     randomToken(48),
		UserID:    userID,
		RoleID:    roleID,
		ExpiresAt: time.Now().Add(s.ttl),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[ticket.Value] = ticket
	return ticket
}

func (s *TicketStore) Consume(value string) (Ticket, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ticket, ok := s.items[value]
	if !ok {
		return Ticket{}, false
	}
	delete(s.items, value)
	if time.Now().After(ticket.ExpiresAt) {
		return Ticket{}, false
	}
	return ticket, true
}

func ctxWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}
