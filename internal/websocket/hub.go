package websocket

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"stox-gateway/internal/workflow"
)

// Hub maintains the set of active clients and broadcasts messages to the clients
type Hub struct {
	// Registered clients
	clients map[*Client]bool

	// Inbound messages from the clients
	broadcast chan []byte

	// Register requests from the clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Logger
	logger *zap.Logger

	// Mutex for thread-safe operations
	mutex sync.RWMutex
}

// Client represents a WebSocket client
type Client struct {
	hub *Hub

	// The websocket connection
	conn *websocket.Conn

	// Buffered channel of outbound messages
	send chan []byte

	// Workflow ID that this client is subscribed to
	workflowID string

	// Client ID for identification
	clientID string
}

// WebSocketMessage represents a message sent over WebSocket
type WebSocketMessage struct {
	Type       string                 `json:"type"`
	WorkflowID string                 `json:"workflow_id"`
	Data       map[string]interface{} `json:"data"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// In production, implement proper origin checking
		return true
	},
}

// NewHub creates a new WebSocket hub
func NewHub(logger *zap.Logger) *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		logger:     logger,
	}
}

// Run starts the hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client] = true
			h.mutex.Unlock()
			
			h.logger.Info("Client connected",
				zap.String("client_id", client.clientID),
				zap.String("workflow_id", client.workflowID),
			)

		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mutex.Unlock()
			
			h.logger.Info("Client disconnected",
				zap.String("client_id", client.clientID),
				zap.String("workflow_id", client.workflowID),
			)

		case message := <-h.broadcast:
			h.mutex.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mutex.RUnlock()
		}
	}
}

// BroadcastWorkflowUpdate sends a workflow update to all subscribed clients
func (h *Hub) BroadcastWorkflowUpdate(workflowID string, eventType string, data map[string]interface{}) {
	message := WebSocketMessage{
		Type:       eventType,
		WorkflowID: workflowID,
		Data:       data,
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		h.logger.Error("Failed to marshal WebSocket message", zap.Error(err))
		return
	}

	h.mutex.RLock()
	for client := range h.clients {
		if client.workflowID == workflowID {
			select {
			case client.send <- messageBytes:
			default:
				close(client.send)
				delete(h.clients, client)
			}
		}
	}
	h.mutex.RUnlock()
}

// ServeWS handles websocket requests from the peer
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	workflowID := r.URL.Query().Get("workflow_id")
	if workflowID == "" {
		http.Error(w, "workflow_id query parameter is required", http.StatusBadRequest)
		return
	}

	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		clientID = "anonymous"
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("WebSocket upgrade failed", zap.Error(err))
		return
	}

	client := &Client{
		hub:        h,
		conn:       conn,
		send:       make(chan []byte, 256),
		workflowID: workflowID,
		clientID:   clientID,
	}

	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines
	go client.writePump()
	go client.readPump()
}

// readPump pumps messages from the websocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.hub.logger.Error("WebSocket error", zap.Error(err))
			}
			break
		}
	}
}

// writePump pumps messages from the hub to the websocket connection
func (c *Client) writePump() {
	defer c.conn.Close()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				c.hub.logger.Error("WebSocket write error", zap.Error(err))
				return
			}
		}
	}
}

// NotifyWorkflowCompleted sends a workflow completed notification
func (h *Hub) NotifyWorkflowCompleted(workflow *workflow.Workflow) {
	data := map[string]interface{}{
		"workflow": workflow,
		"message":  "Workflow completed successfully",
	}
	h.BroadcastWorkflowUpdate(workflow.ID, "workflow_completed", data)
}

// NotifyWorkflowFailed sends a workflow failed notification
func (h *Hub) NotifyWorkflowFailed(workflow *workflow.Workflow, errorMsg string) {
	data := map[string]interface{}{
		"workflow":  workflow,
		"error":     errorMsg,
		"message":   "Workflow failed",
	}
	h.BroadcastWorkflowUpdate(workflow.ID, "workflow_failed", data)
}

// NotifyStepCompleted sends a step completed notification
func (h *Hub) NotifyStepCompleted(workflow *workflow.Workflow, stepID string, result map[string]interface{}) {
	data := map[string]interface{}{
		"workflow": workflow,
		"step_id":  stepID,
		"result":   result,
		"message":  "Step completed",
	}
	h.BroadcastWorkflowUpdate(workflow.ID, "step_completed", data)
}
