package service

import (
	"fmt"
	"sync"
	"time"

	executorV1 "github.com/go-tangra/go-tangra-executor/gen/go/executor/service/v1"
)

const commandChannelBufferSize = 16

// connectedClient holds the command channel and metadata for a connected client.
type connectedClient struct {
	ch          chan *executorV1.ExecutionCommand
	version     string
	connectedAt time.Time
}

// ConnectedClientInfo is a read-only snapshot of a connected client's metadata.
type ConnectedClientInfo struct {
	ClientID    string
	Version     string
	ConnectedAt time.Time
}

// CommandRegistry manages in-memory command channels for connected clients.
type CommandRegistry struct {
	mu      sync.RWMutex
	clients map[string]*connectedClient
}

// NewCommandRegistry creates a new CommandRegistry.
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		clients: make(map[string]*connectedClient),
	}
}

// Register creates a buffered channel for the given client.
// If one already exists, it is closed first.
func (r *CommandRegistry) Register(clientID, version string) <-chan *executorV1.ExecutionCommand {
	r.mu.Lock()
	defer r.mu.Unlock()

	if old, ok := r.clients[clientID]; ok {
		close(old.ch)
	}
	ch := make(chan *executorV1.ExecutionCommand, commandChannelBufferSize)
	r.clients[clientID] = &connectedClient{
		ch:          ch,
		version:     version,
		connectedAt: time.Now(),
	}
	return ch
}

// Unregister closes and removes the channel for the given client.
func (r *CommandRegistry) Unregister(clientID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if c, ok := r.clients[clientID]; ok {
		close(c.ch)
		delete(r.clients, clientID)
	}
}

// Send sends an execution command to a connected client.
// Returns an error if the client is not connected or the channel is full.
func (r *CommandRegistry) Send(clientID string, cmd *executorV1.ExecutionCommand) error {
	r.mu.RLock()
	c, ok := r.clients[clientID]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("client %s not connected", clientID)
	}

	select {
	case c.ch <- cmd:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout sending command to client %s", clientID)
	}
}

// IsConnected checks whether a client has an active channel.
func (r *CommandRegistry) IsConnected(clientID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.clients[clientID]
	return ok
}

// ListConnected returns a snapshot of all currently connected clients.
func (r *CommandRegistry) ListConnected() []ConnectedClientInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ConnectedClientInfo, 0, len(r.clients))
	for id, c := range r.clients {
		result = append(result, ConnectedClientInfo{
			ClientID:    id,
			Version:     c.version,
			ConnectedAt: c.connectedAt,
		})
	}
	return result
}
