// Phase 11 - File: internal/interfaces/http/handlers/ws_handler_test.go
// 实现 WebSocket 实时事件通知端点的集成测试。
//
// 实现要求:
//   - 测试 WebSocket 连接升级
//   - 测试消息广播（多个客户端接收）
//   - 测试客户端注册/注销
//   - 测试 ping/pong 心跳
//   - 测试连接关闭清理
//   - 使用 gorilla/websocket 客户端连接测试服务器
//   - 强制约束：文件最后一行必须为 //Personal.AI order the ending

package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

const (
	testReadTimeout = 5 * time.Second
	pollInterval    = 50 * time.Millisecond
	pollTimeout     = 3 * time.Second
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// setupWSTestServer creates a test HTTP server with a WSHandler wired to a
// no-op logger.  The server is automatically closed when the test finishes.
func setupWSTestServer(t *testing.T) (*httptest.Server, *WSHandler) {
	t.Helper()

	logger := logging.NewNopLogger()
	handler := NewWSHandler(logger)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return server, handler
}

// wsURL converts an httptest server http:// URL into a ws:// URL suitable
// for gorilla/websocket dialler.
func wsURL(server *httptest.Server, path string) string {
	return "ws" + strings.TrimPrefix(server.URL, "http") + path
}

// mustDialWebSocket connects to the WebSocket endpoint and registers cleanup.
func mustDialWebSocket(t *testing.T, server *httptest.Server) *websocket.Conn {
	t.Helper()

	u := wsURL(server, "/api/v1/ws/events")
	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err, "WebSocket dial should succeed")

	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// readMessageWithTimeout reads one message from conn or fails the test if the
// deadline elapses.
func readMessageWithTimeout(t *testing.T, conn *websocket.Conn, timeout time.Duration) (int, []byte) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	msgType, data, err := conn.ReadMessage()
	require.NoError(t, err, "should read message within timeout")
	return msgType, data
}

// pollClientCount keeps calling handler.clientCount() until it reaches the
// wanted value or pollTimeout elapses.
func pollClientCount(t *testing.T, handler *WSHandler, want int) {
	t.Helper()
	deadline := time.Now().Add(pollTimeout)
	for {
		if handler.clientCount() == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for clientCount == %d (got %d)",
				want, handler.clientCount())
		}
		time.Sleep(pollInterval)
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestWebSocketUpgrade verifies that a normal HTTP GET request to the
// WebSocket endpoint is upgraded, and that the handler registers exactly one
// client.
func TestWebSocketUpgrade(t *testing.T) {
	server, handler := setupWSTestServer(t)
	conn := mustDialWebSocket(t, server)
	defer conn.Close()

	pollClientCount(t, handler, 1)

	// Verify the connection is bi-directionally functional by broadcasting a
	// message and reading it on the client.
	handler.BroadcastEvent(EventTypeSystemNotification, "upgrade ok")

	_, data := readMessageWithTimeout(t, conn, testReadTimeout)
	var msg WSMessage
	err := json.Unmarshal(data, &msg)
	require.NoError(t, err, "broadcast message should be valid JSON")
	assert.Equal(t, EventTypeSystemNotification, msg.Type)
	assert.NotEmpty(t, msg.Timestamp, "timestamp should be set")
}

// TestWebSocketBroadcast verifies that a message broadcast by the server
// reaches every connected WebSocket client.
func TestWebSocketBroadcast(t *testing.T) {
	server, handler := setupWSTestServer(t)

	const numClients = 3
	conns := make([]*websocket.Conn, numClients)
	for i := 0; i < numClients; i++ {
		conns[i] = mustDialWebSocket(t, server)
	}
	pollClientCount(t, handler, numClients)

	// Broadcast a structured message.
	msg := WSMessage{
		Type: EventTypePatentMatch,
		Payload: map[string]interface{}{
			"patent_id": "US-12345",
			"score":     0.95,
		},
		Timestamp: time.Now().UTC(),
	}
	handler.Broadcast(msg)

	// Every client should receive the same message.
	for i, conn := range conns {
		_, data := readMessageWithTimeout(t, conn, testReadTimeout)

		var received WSMessage
		err := json.Unmarshal(data, &received)
		require.NoErrorf(t, err, "client %d: should decode JSON", i)
		assert.Equalf(t, msg.Type, received.Type, "client %d: type should match", i)
		assert.NotEmptyf(t, received.Timestamp, "client %d: timestamp should be set", i)

		// Verify the nested payload.
		payloadMap, ok := received.Payload.(map[string]interface{})
		require.Truef(t, ok, "client %d: payload should be a map", i)
		assert.Equalf(t, "US-12345", payloadMap["patent_id"], "client %d: patent_id", i)
	}
}

// TestWebSocketBroadcastEvent verifies that the BroadcastEvent convenience
// method correctly wraps payload in a WSMessage and broadcasts it.
func TestWebSocketBroadcastEvent(t *testing.T) {
	server, handler := setupWSTestServer(t)
	conn := mustDialWebSocket(t, server)
	defer conn.Close()

	pollClientCount(t, handler, 1)

	handler.BroadcastEvent(EventTypeDeadlineAlert, map[string]string{
		"patent_id": "US-99999",
		"due_date":  "2025-06-01",
	})

	_, data := readMessageWithTimeout(t, conn, testReadTimeout)
	var msg WSMessage
	err := json.Unmarshal(data, &msg)
	require.NoError(t, err)
	assert.Equal(t, EventTypeDeadlineAlert, msg.Type)
	assert.NotEmpty(t, msg.Timestamp)
}

// TestWebSocketClientLifecycle validates client registration and
// deregistration by observing handler.clientCount() as clients connect and
// disconnect.
func TestWebSocketClientLifecycle(t *testing.T) {
	server, handler := setupWSTestServer(t)

	// Start with zero clients.
	assert.Equal(t, 0, handler.clientCount(), "initial count should be 0")

	// Connect two clients in sequence.
	conn1 := mustDialWebSocket(t, server)
	pollClientCount(t, handler, 1)

	conn2 := mustDialWebSocket(t, server)
	pollClientCount(t, handler, 2)

	// Close the second client; count should drop.
	conn2.Close()
	pollClientCount(t, handler, 1)

	// Close the first client; count should return to 0.
	conn1.Close()
	pollClientCount(t, handler, 0)

	// Reconnect to verify the handler still accepts new clients after all
	// previous ones have gone.
	conn3 := mustDialWebSocket(t, server)
	pollClientCount(t, handler, 1)
	_ = conn3
}

// TestWebSocketPingPong verifies that the pong handler is correctly
// configured so that receipt of a WebSocket Pong frame keeps the connection
// alive and functional.
//
// The server's writePump sends Ping frames at pingInterval; the gorilla/
// websocket client library auto-responds with Pong.  We simulate the same
// round-trip by writing a Pong control frame from the client side and then
// verify the server still delivers broadcast messages.
func TestWebSocketPingPong(t *testing.T) {
	server, handler := setupWSTestServer(t)
	conn := mustDialWebSocket(t, server)
	defer conn.Close()

	pollClientCount(t, handler, 1)

	// Write a Pong control message from the client to simulate the client
	// side of the heartbeat protocol.  This should trigger the server's
	// SetPongHandler, which extends the read deadline.
	err := conn.WriteControl(websocket.PongMessage, []byte("heartbeat"),
		time.Now().Add(time.Second))
	require.NoError(t, err, "should write Pong control frame")

	// Give the server a moment to process the control frame.
	time.Sleep(100 * time.Millisecond)

	// The connection should still be usable for broadcast delivery.
	handler.BroadcastEvent(EventTypeSystemNotification, "pong received")

	_, data := readMessageWithTimeout(t, conn, testReadTimeout)
	var msg WSMessage
	err = json.Unmarshal(data, &msg)
	require.NoError(t, err)
	assert.Equal(t, EventTypeSystemNotification, msg.Type)
}

// TestWebSocketConnectionCleanup verifies that when a client closes the
// connection abruptly the server-side goroutines complete and the client is
// removed from the registry.
func TestWebSocketConnectionCleanup(t *testing.T) {
	server, handler := setupWSTestServer(t)
	conn := mustDialWebSocket(t, server)
	pollClientCount(t, handler, 1)

	// Close the underlying TCP connection abruptly (no close handshake).
	conn.Close()

	// The readPump should detect the error and call removeClient.
	pollClientCount(t, handler, 0)

	// Verify no panic occurs on subsequent broadcast after all clients have
	// been removed.
	handler.BroadcastEvent(EventTypeSystemNotification, "cleanup done")
}

// TestWebSocketConcurrentBroadcast stresses the handler with broadcasts from
// multiple goroutines to catch any data races in the sync.Map or send
// channels.
func TestWebSocketConcurrentBroadcast(t *testing.T) {
	server, handler := setupWSTestServer(t)

	const numConnections = 5
	conns := make([]*websocket.Conn, numConnections)
	for i := 0; i < numConnections; i++ {
		conns[i] = mustDialWebSocket(t, server)
	}
	pollClientCount(t, handler, numConnections)

	// Launch concurrent broadcasters.
	const numMessages = 20
	var wg sync.WaitGroup
	for i := 0; i < numMessages; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			handler.Broadcast(WSMessage{
				Type:      EventTypeInfringementWarning,
				Payload:   map[string]int{"seq": idx},
				Timestamp: time.Now().UTC(),
			})
		}(i)
	}
	wg.Wait()

	// Each client should have received exactly numMessages messages.
	for i, conn := range conns {
		for j := 0; j < numMessages; j++ {
			_, data := readMessageWithTimeout(t, conn, testReadTimeout)
			var msg WSMessage
			err := json.Unmarshal(data, &msg)
			require.NoErrorf(t, err, "client %d msg %d: should decode JSON", i, j)
			assert.Equal(t, EventTypeInfringementWarning, msg.Type)
		}
	}
}

// TestWebSocketMessageTypes verifies that every event type constant produces
// a correctly formatted message.
func TestWebSocketMessageTypes(t *testing.T) {
	server, handler := setupWSTestServer(t)
	conn := mustDialWebSocket(t, server)
	defer conn.Close()

	pollClientCount(t, handler, 1)

	types := []string{
		EventTypePatentMatch,
		EventTypeDeadlineAlert,
		EventTypeInfringementWarning,
		EventTypeSystemNotification,
	}

	for _, et := range types {
		handler.BroadcastEvent(et, "test-"+et)
	}

	for _, et := range types {
		_, data := readMessageWithTimeout(t, conn, testReadTimeout)
		var msg WSMessage
		err := json.Unmarshal(data, &msg)
		require.NoError(t, err)
		assert.Equal(t, et, msg.Type, "event type should match %q", et)
	}
}

// TestWebSocketUpgradeRejection verifies that a plain (non-WebSocket) HTTP
// request to the endpoint does NOT get upgraded and returns an appropriate
// HTTP error status.
func TestWebSocketUpgradeRejection(t *testing.T) {
	server, _ := setupWSTestServer(t)

	// Perform a regular HTTP GET (no WebSocket upgrade headers).
	resp, err := http.Get(server.URL + "/api/v1/ws/events")
	require.NoError(t, err)
	defer resp.Body.Close()

	// The server should reject the non-WebSocket request.
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
		"plain HTTP request to WS endpoint should be rejected")
}

// TestWebSocketNonExistentEndpoint verifies that a request to a path that is
// not registered gets a 404.
func TestWebSocketNonExistentEndpoint(t *testing.T) {
	server, _ := setupWSTestServer(t)
	resp, err := http.Get(server.URL + "/api/v1/ws/nonexistent")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode,
		"unregistered WS path should return 404")
}

// TestWebSocketDisconnectDuringBroadcast verifies that concurrently
// broadcasting and disconnecting clients does not cause a panic or hang.
func TestWebSocketDisconnectDuringBroadcast(t *testing.T) {
	server, handler := setupWSTestServer(t)

	// Connect several clients and immediately close some while broadcasting.
	dying := mustDialWebSocket(t, server)
	survivor := mustDialWebSocket(t, server)
	pollClientCount(t, handler, 2)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			handler.Broadcast(WSMessage{
				Type:      EventTypeSystemNotification,
				Payload:   "stress",
				Timestamp: time.Now().UTC(),
			})
		}()
	}

	// Close the first client while broadcasts are in flight.
	dying.Close()
	wg.Wait()

	// Wait for the dying client's cleanup goroutine to complete.
	pollClientCount(t, handler, 1)

	// The surviving client should still be registered.
	handler.BroadcastEvent(EventTypeSystemNotification, "after-stress")
	_, data := readMessageWithTimeout(t, survivor, testReadTimeout)
	var msg WSMessage
	err := json.Unmarshal(data, &msg)
	assert.NoError(t, err)
	assert.Equal(t, EventTypeSystemNotification, msg.Type)
}

//Personal.AI order the ending
