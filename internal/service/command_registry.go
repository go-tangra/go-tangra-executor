package service

import (
	"fmt"
	"sync"
	"time"

	executorV1 "github.com/go-tangra/go-tangra-executor/gen/go/executor/service/v1"
)

const commandChannelBufferSize = 16

// CommandRegistry manages in-memory command channels for connected clients.
type CommandRegistry struct {
	mu       sync.RWMutex
	channels map[string]chan *executorV1.ExecutionCommand
}

// NewCommandRegistry creates a new CommandRegistry.
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		channels: make(map[string]chan *executorV1.ExecutionCommand),
	}
}

// Register creates a buffered channel for the given client.
// If one already exists, it is closed first.
func (r *CommandRegistry) Register(clientID string) <-chan *executorV1.ExecutionCommand {
	r.mu.Lock()
	defer r.mu.Unlock()

	if old, ok := r.channels[clientID]; ok {
		close(old)
	}
	ch := make(chan *executorV1.ExecutionCommand, commandChannelBufferSize)
	r.channels[clientID] = ch
	return ch
}

// Unregister closes and removes the channel for the given client.
func (r *CommandRegistry) Unregister(clientID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if ch, ok := r.channels[clientID]; ok {
		close(ch)
		delete(r.channels, clientID)
	}
}

// Send sends an execution command to a connected client.
// Returns an error if the client is not connected or the channel is full.
func (r *CommandRegistry) Send(clientID string, cmd *executorV1.ExecutionCommand) error {
	r.mu.RLock()
	ch, ok := r.channels[clientID]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("client %s not connected", clientID)
	}

	select {
	case ch <- cmd:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout sending command to client %s", clientID)
	}
}

// IsConnected checks whether a client has an active channel.
func (r *CommandRegistry) IsConnected(clientID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.channels[clientID]
	return ok
}
