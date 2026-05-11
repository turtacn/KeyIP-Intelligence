// Phase 11 - File: internal/interfaces/http/handlers/ws_handler.go
// 实现 WebSocket 实时事件通知端点。
//
// 实现要求:
//   - 功能定位：提供 WebSocket 实时推送端点，支持专利匹配、截止日期告警、
//     侵权警告和系统通知等事件类型的实时广播。
//   - 核心实现：
//   - GET /api/v1/ws/events → WebSocket 升级
//   - 客户端注册/注销管理（sync.Map）
//   - 广播消息给所有连接的客户端
//   - Ping/Pong 心跳（30s 间隔，60s 超时）
//   - 消息类型：patent_match、deadline_alert、infringement_warning、system_notification
//   - 使用 gorilla/websocket 库
//   - 业务逻辑：
//   - 只读 WebSocket（单向推送）：服务端 → 客户端，不处理客户端上行消息
//   - 发送缓冲满时自动断开慢速客户端
//   - 连接断开时自动清理资源
//   - 强制约束：文件最后一行必须为 //Personal.AI order the ending

package handlers

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// WebSocket event type constants for frontend consumption.
const (
	EventTypePatentMatch         = "patent_match"
	EventTypeDeadlineAlert       = "deadline_alert"
	EventTypeInfringementWarning = "infringement_warning"
	EventTypeSystemNotification  = "system_notification"
)

// WebSocket operational constants.
const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingInterval   = 30 * time.Second
	maxMessageSize = 4096
)

// WSMessage represents a structured message sent over WebSocket connections.
type WSMessage struct {
	Type      string      `json:"type"`
	Payload   interface{} `json:"payload,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// WSHandler manages WebSocket connections, including client registration,
// deregistration, and broadcasting of real-time events.
type WSHandler struct {
	upgrader websocket.Upgrader
	clients  sync.Map
	logger   logging.Logger
}

// wsClient represents a single connected WebSocket client.
type wsClient struct {
	mu      sync.Mutex
	closed  bool
	handler *WSHandler
	conn    *websocket.Conn
	send    chan []byte
}

// NewWSHandler creates a new WSHandler with the given logger.
func NewWSHandler(logger logging.Logger) *WSHandler {
	return &WSHandler{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins; tighten for production environments
			},
		},
		logger: logger,
	}
}

// RegisterRoutes registers the WebSocket endpoint on the given mux.
func (h *WSHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/ws/events", h.ServeWS)
}

// ServeWS handles the WebSocket upgrade request from an HTTP connection.
func (h *WSHandler) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed",
			logging.Err(err),
			logging.String("remote_addr", r.RemoteAddr))
		return
	}

	c := &wsClient{
		handler: h,
		conn:    conn,
		send:    make(chan []byte, 256),
	}

	h.clients.Store(c, true)
	h.logger.Info("websocket client connected",
		logging.String("remote_addr", r.RemoteAddr),
		logging.Int("total_clients", h.clientCount()))

	// Start read and write pumps in separate goroutines.
	go c.writePump()
	go c.readPump()
}

// Broadcast sends a message to all connected WebSocket clients.
// If a client's send buffer is full, the client is removed.
func (h *WSHandler) Broadcast(msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("websocket broadcast marshal failed", logging.Err(err))
		return
	}

	h.clients.Range(func(key, value interface{}) bool {
		c, ok := key.(*wsClient)
		if !ok {
			return true
		}

		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return true
		}
		select {
		case c.send <- data:
			c.mu.Unlock()
		default:
			// Client's send buffer is full; drop the slow client.
			c.mu.Unlock()
			h.logger.Warn("websocket client send buffer full, dropping client")
			h.removeClient(c)
		}
		return true
	})
}

// BroadcastEvent is a convenience method to broadcast a typed event with payload.
func (h *WSHandler) BroadcastEvent(eventType string, payload interface{}) {
	h.Broadcast(WSMessage{
		Type:      eventType,
		Payload:   payload,
		Timestamp: time.Now().UTC(),
	})
}

// clientCount returns the number of currently connected WebSocket clients.
func (h *WSHandler) clientCount() int {
	count := 0
	h.clients.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

// removeClient removes a client from the registry and signals the write pump
// to close the connection by closing the send channel. It is safe to call
// concurrently from multiple goroutines.
func (h *WSHandler) removeClient(c *wsClient) {
	h.clients.Delete(c)
	c.mu.Lock()
	if !c.closed {
		close(c.send)
		c.closed = true
	}
	c.mu.Unlock()
}

// readPump reads messages from the WebSocket connection.
// In this server-to-client push architecture, incoming client messages are
// discarded; the primary purpose is to detect connection closure and handle
// pong responses for keep-alive.
func (c *wsClient) readPump() {
	defer func() {
		c.handler.removeClient(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.handler.logger.Warn("websocket read error",
					logging.Err(err),
					logging.String("remote_addr", c.conn.RemoteAddr().String()))
			}
			break
		}
		// Incoming client messages are intentionally not processed.
		// This WebSocket endpoint is designed for server-to-client push only.
	}
}

// writePump writes messages from the send channel to the WebSocket connection
// and sends periodic pings to keep the connection alive.
//
// Ping interval is 30 seconds. If a pong is not received within 60 seconds
// (pongWait), the read deadline will cause the readPump to close the connection.
func (c *wsClient) writePump() {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if !ok {
				// The send channel was closed by removeClient; send close frame.
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			if _, err := w.Write(message); err != nil {
				_ = w.Close()
				return
			}
			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

//Personal.AI order the ending
