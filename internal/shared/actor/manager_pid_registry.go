package actor

import (
	"sync"

	protoactor "github.com/asynkron/protoactor-go/actor"
)

type ManagerPIDKey string

const (
	ManagerPIDWorld    ManagerPIDKey = "world_manager"
	ManagerPIDPlayer   ManagerPIDKey = "player_manager"
	ManagerPIDAlliance ManagerPIDKey = "alliance_manager"
)

type ManagerPIDResolver interface {
	ResolveManagerPID(key ManagerPIDKey) (*protoactor.PID, bool)
}

type ManagerPIDRegistry interface {
	ManagerPIDResolver
	RegisterManagerPID(key ManagerPIDKey, pid *protoactor.PID)
}

type PIDRegistry struct {
	mu   sync.RWMutex
	pids map[ManagerPIDKey]*protoactor.PID
}

func NewPIDRegistry() *PIDRegistry {
	return &PIDRegistry{
		pids: make(map[ManagerPIDKey]*protoactor.PID),
	}
}

func (r *PIDRegistry) RegisterManagerPID(key ManagerPIDKey, pid *protoactor.PID) {
	if r == nil || key == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if pid == nil {
		delete(r.pids, key)
		return
	}
	r.pids[key] = pid
}

func (r *PIDRegistry) ResolveManagerPID(key ManagerPIDKey) (*protoactor.PID, bool) {
	if r == nil || key == "" {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	pid, ok := r.pids[key]
	return pid, ok && pid != nil
}
