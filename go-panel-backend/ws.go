package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type NodeHub struct {
	server *Server
	mu     sync.RWMutex
	nodes  map[int64]*NodeSession
	admins map[*AdminSession]struct{}
}

type NodeSession struct {
	hub        *NodeHub
	node       Node
	secret     string
	conn       *websocket.Conn
	send       chan []byte
	pendingMu  sync.Mutex
	pending    map[string]chan NodeCommandResponse
	cancelFunc context.CancelFunc
}

type AdminSession struct {
	hub    *NodeHub
	userID int64
	conn   *websocket.Conn
	send   chan []byte
}

func NewNodeHub(server *Server) *NodeHub {
	return &NodeHub{
		server: server,
		nodes:  map[int64]*NodeSession{},
		admins: map[*AdminSession]struct{}{},
	}
}

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (s *Server) handleSystemInfoWebSocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errResp(-1, "method not allowed"))
		return
	}
	socketType := strings.TrimSpace(r.URL.Query().Get("type"))
	if socketType == "1" {
		s.handleNodeSocket(w, r)
		return
	}
	s.handleAdminSocket(w, r)
}

func (s *Server) handleAdminSocket(w http.ResponseWriter, r *http.Request) {
	ticketValue := strings.TrimSpace(r.URL.Query().Get("ticket"))
	ticket, ok := s.ticketStore.Consume(ticketValue)
	if !ok || ticket.RoleID != 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID := ticket.UserID

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	session := &AdminSession{
		hub:    s.nodeHub,
		userID: userID,
		conn:   conn,
		send:   make(chan []byte, 32),
	}

	s.nodeHub.addAdmin(session)
	go session.writeLoop()
	go session.readLoop()
}

func (s *Server) handleNodeSocket(w http.ResponseWriter, r *http.Request) {
	secret := strings.TrimSpace(r.Header.Get("X-Node-Secret"))
	if secret == "" {
		secret = strings.TrimSpace(r.URL.Query().Get("secret"))
	}
	if secret == "" {
		http.Error(w, "missing node secret", http.StatusUnauthorized)
		return
	}

	node, err := s.getNodeBySecret(secret)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	version := strings.TrimSpace(r.URL.Query().Get("version"))
	httpValue := parseIntDefault(r.URL.Query().Get("http"), node.HTTP)
	tlsValue := parseIntDefault(r.URL.Query().Get("tls"), node.TLS)
	socksValue := parseIntDefault(r.URL.Query().Get("socks"), node.Socks)

	now := time.Now().UnixMilli()
	_, _ = s.db.Exec(`UPDATE node SET status = 1, version = ?, http = ?, tls = ?, socks = ?, updated_time = ? WHERE id = ?`,
		version, httpValue, tlsValue, socksValue, now, node.ID)
	node.Status = 1
	node.Version = version
	node.HTTP = httpValue
	node.TLS = tlsValue
	node.Socks = socksValue

	ctx, cancel := context.WithCancel(context.Background())
	session := &NodeSession{
		hub:        s.nodeHub,
		node:       node,
		secret:     secret,
		conn:       conn,
		send:       make(chan []byte, 64),
		pending:    map[string]chan NodeCommandResponse{},
		cancelFunc: cancel,
	}
	s.nodeHub.addNode(session)
	s.nodeHub.broadcastStatus(node.ID, 1)

	go session.writeLoop(ctx)
	go session.readLoop(ctx)
}

func (h *NodeHub) addNode(session *NodeSession) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if existing := h.nodes[session.node.ID]; existing != nil {
		existing.close()
	}
	h.nodes[session.node.ID] = session
}

func (h *NodeHub) removeNode(session *NodeSession) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if current := h.nodes[session.node.ID]; current == session {
		delete(h.nodes, session.node.ID)
	}
}

func (h *NodeHub) addAdmin(session *AdminSession) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.admins[session] = struct{}{}
}

func (h *NodeHub) removeAdmin(session *AdminSession) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.admins, session)
}

func (h *NodeHub) broadcastStatus(nodeID int64, status int) {
	message, _ := json.Marshal(map[string]interface{}{
		"id":   nodeID,
		"type": "status",
		"data": status,
	})
	h.broadcast(message)
}

func (h *NodeHub) broadcastInfo(nodeID int64, data interface{}) {
	message, _ := json.Marshal(map[string]interface{}{
		"id":   nodeID,
		"type": "info",
		"data": data,
	})
	h.broadcast(message)
}

func (h *NodeHub) broadcast(payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for admin := range h.admins {
		select {
		case admin.send <- payload:
		default:
		}
	}
}

func (s *AdminSession) readLoop() {
	defer func() {
		s.hub.removeAdmin(s)
		_ = s.conn.Close()
	}()
	for {
		if _, _, err := s.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (s *AdminSession) writeLoop() {
	defer func() {
		s.hub.removeAdmin(s)
		_ = s.conn.Close()
	}()
	for payload := range s.send {
		_ = s.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := s.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			return
		}
	}
}

func (s *NodeSession) close() {
	if s.cancelFunc != nil {
		s.cancelFunc()
	}
	_ = s.conn.Close()
}

func (s *NodeSession) readLoop(ctx context.Context) {
	defer func() {
		s.hub.removeNode(s)
		s.cancelFunc()
		_ = s.conn.Close()
		now := time.Now().UnixMilli()
		_, _ = s.hub.server.db.Exec(`UPDATE node SET status = 0, updated_time = ? WHERE id = ?`, now, s.node.ID)
		s.hub.broadcastStatus(s.node.ID, 0)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_ = s.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, payload, err := s.conn.ReadMessage()
		if err != nil {
			return
		}
		decoded, err := s.decodeMessage(payload)
		if err != nil {
			log.Printf("node message decode failed: %v", err)
			continue
		}

		var response NodeCommandResponse
		if err := json.Unmarshal(decoded, &response); err == nil && response.RequestID != "" {
			s.pendingMu.Lock()
			ch := s.pending[response.RequestID]
			delete(s.pending, response.RequestID)
			s.pendingMu.Unlock()
			if ch != nil {
				ch <- response
			}
			continue
		}

		var info map[string]interface{}
		if err := json.Unmarshal(decoded, &info); err == nil {
			if _, ok := info["memory_usage"]; ok {
				s.hub.broadcastInfo(s.node.ID, info)
			}
		}
	}
}

func (s *NodeSession) decodeMessage(payload []byte) ([]byte, error) {
	var envelope EncryptedEnvelope
	if err := json.Unmarshal(payload, &envelope); err == nil && envelope.Encrypted && envelope.Data != "" {
		return decryptSecretPayload(s.secret, envelope.Data)
	}
	return payload, nil
}

func (s *NodeSession) encodeMessage(payload []byte) ([]byte, error) {
	encrypted, err := encryptSecretPayload(s.secret, payload)
	if err != nil {
		return nil, err
	}
	wrapper := EncryptedEnvelope{
		Encrypted: true,
		Data:      encrypted,
		Timestamp: time.Now().Unix(),
	}
	return json.Marshal(wrapper)
}

func (s *NodeSession) writeLoop(ctx context.Context) {
	defer func() {
		_ = s.conn.Close()
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case payload := <-s.send:
			_ = s.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := s.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		}
	}
}

func (s *Server) sendNodeCommand(nodeID int64, commandType string, data interface{}) GostResponse {
	s.nodeHub.mu.RLock()
	session := s.nodeHub.nodes[nodeID]
	s.nodeHub.mu.RUnlock()
	if session == nil {
		return GostResponse{Code: -1, Msg: "节点不在线"}
	}

	requestID := randomToken(24)
	command := NodeCommand{
		Type:      commandType,
		Data:      data,
		RequestID: requestID,
	}
	raw, err := json.Marshal(command)
	if err != nil {
		return GostResponse{Code: -1, Msg: err.Error()}
	}
	wire, err := session.encodeMessage(raw)
	if err != nil {
		return GostResponse{Code: -1, Msg: err.Error()}
	}

	respChan := make(chan NodeCommandResponse, 1)
	session.pendingMu.Lock()
	session.pending[requestID] = respChan
	session.pendingMu.Unlock()

	select {
	case session.send <- wire:
	default:
		return GostResponse{Code: -1, Msg: "节点发送队列已满"}
	}

	ctx, cancel := ctxWithTimeout(s.cfg.NodeCommandTimeout)
	defer cancel()

	select {
	case <-ctx.Done():
		session.pendingMu.Lock()
		delete(session.pending, requestID)
		session.pendingMu.Unlock()
		return GostResponse{Code: -1, Msg: "等待节点响应超时"}
	case response := <-respChan:
		if response.Success {
			return GostResponse{Code: 0, Msg: response.Message, Data: response.Data}
		}
		return GostResponse{Code: -1, Msg: response.Message, Data: response.Data}
	}
}

func parseIntDefault(value string, fallback int) int {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}
