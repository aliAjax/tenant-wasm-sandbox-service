package domain

import (
	"errors"
	"time"
)

type NodeState string

const (
	NodeReady    NodeState = "ready"
	NodeDraining NodeState = "draining"
	NodeIsolated NodeState = "isolated"
	NodeStopped  NodeState = "stopped"
)

var ErrNodeUnavailable = errors.New("node unavailable")

type Node struct {
	ID        string    `json:"id"`
	State     NodeState `json:"state"`
	Active    int       `json:"active"`
	Capacity  int       `json:"capacity"`
	StartedAt time.Time `json:"started_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Reason    string    `json:"reason,omitempty"`
}
