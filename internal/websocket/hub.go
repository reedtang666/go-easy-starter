package websocket

import (
	"context"
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/reedtang666/go-easy-starter/internal/config"
	"github.com/reedtang666/go-easy-starter/internal/library/contexts"
	"net/http"
	"sync"
	"time"
)

type peer struct {
	id   contexts.Identity
	conn *websocket.Conn
	out  chan []byte
	done chan struct{}
	once sync.Once
}

func (p *peer) stop() { p.once.Do(func() { close(p.done); p.conn.Close() }) }

type Hub struct {
	cfg      config.Config
	tickets  *Tickets
	validate func(context.Context, contexts.Identity) error
	mu       sync.RWMutex
	peers    map[*peer]struct{}
	closing  bool
	wg       sync.WaitGroup
}

func NewHub(c config.Config, t *Tickets, validate func(context.Context, contexts.Identity) error) *Hub {
	return &Hub{cfg: c, tickets: t, validate: validate, peers: map[*peer]struct{}{}}
}
func (h *Hub) origin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	for _, v := range h.cfg.Server.Origins {
		if v == origin {
			return true
		}
	}
	return false
}
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request) {
	if !h.origin(r) {
		http.Error(w, "origin denied", 403)
		return
	}
	id, e := h.tickets.Consume(r.Context(), r.URL.Query().Get("ticket"))
	if e != nil {
		http.Error(w, "invalid connection ticket", 401)
		return
	}
	if e = h.validate(r.Context(), id); e != nil {
		http.Error(w, "session expired", 401)
		return
	}
	h.mu.Lock()
	if h.closing || len(h.peers) >= h.cfg.WebSocket.MaxConnections {
		h.mu.Unlock()
		http.Error(w, "connection limit reached", 503)
		return
	}
	up := websocket.Upgrader{CheckOrigin: h.origin, HandshakeTimeout: 5 * time.Second}
	conn, e := up.Upgrade(w, r, nil)
	if e != nil {
		h.mu.Unlock()
		return
	}
	p := &peer{id: id, conn: conn, out: make(chan []byte, 16), done: make(chan struct{})}
	h.peers[p] = struct{}{}
	h.wg.Add(2)
	h.mu.Unlock()
	defer func() { p.stop(); h.mu.Lock(); delete(h.peers, p); h.mu.Unlock(); h.wg.Done() }()
	go h.write(p)
	interval := time.Duration(h.cfg.WebSocket.HeartbeatSeconds) * time.Second
	conn.SetReadLimit(h.cfg.WebSocket.MaxMessageBytes)
	conn.SetReadDeadline(time.Now().Add(interval * 2))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(interval * 2)) })
	for {
		typ, b, e := conn.ReadMessage()
		if e != nil {
			return
		}
		if typ != websocket.TextMessage {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		e = h.validate(ctx, id)
		cancel()
		if e != nil {
			return
		}
		var message struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if json.Unmarshal(b, &message) != nil {
			return
		}
		if message.Type != "echo" && message.Type != "heartbeat" {
			return
		}
		select {
		case p.out <- b:
		case <-p.done:
			return
		default:
			return
		}
	}
}
func (h *Hub) write(p *peer) {
	defer h.wg.Done()
	defer p.stop()
	ticker := time.NewTicker(time.Duration(h.cfg.WebSocket.HeartbeatSeconds) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-p.done:
			return
		case b := <-p.out:
			p.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if p.conn.WriteMessage(websocket.TextMessage, b) != nil {
				return
			}
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			e := h.validate(ctx, p.id)
			cancel()
			if e != nil {
				return
			}
			p.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if p.conn.WriteMessage(websocket.PingMessage, nil) != nil {
				return
			}
		}
	}
}
func (h *Hub) SendUser(userID int64, message any) error {
	b, e := json.Marshal(message)
	if e != nil {
		return e
	}
	h.mu.RLock()
	peers := []*peer{}
	for p := range h.peers {
		if p.id.UserID == userID {
			peers = append(peers, p)
		}
	}
	h.mu.RUnlock()
	for _, p := range peers {
		select {
		case p.out <- b:
		case <-p.done:
		default:
			p.stop()
		}
	}
	return nil
}
func (h *Hub) Close() {
	h.mu.Lock()
	h.closing = true
	peers := []*peer{}
	for p := range h.peers {
		peers = append(peers, p)
	}
	h.mu.Unlock()
	for _, p := range peers {
		p.stop()
	}
	h.wg.Wait()
}
