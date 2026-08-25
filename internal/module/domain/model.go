package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

type State string

const (
	StateDraft     State = "draft"
	StatePublished State = "published"
	StateWithdrawn State = "withdrawn"
)

var (
	ErrNotFound          = errors.New("module not found")
	ErrImmutable         = errors.New("published module is immutable")
	ErrInvalidDigest     = errors.New("module digest mismatch")
	ErrIncompatibleABI   = errors.New("incompatible ABI")
	ErrInvalidTransition = errors.New("invalid module state transition")
)

type Capability struct {
	Name   string `json:"name"`
	Access string `json:"access"`
}

type Module struct {
	ID           string       `json:"id"`
	TenantID     string       `json:"tenant_id"`
	Name         string       `json:"name"`
	Version      string       `json:"version"`
	Digest       string       `json:"digest"`
	Entry        string       `json:"entry"`
	ABI          string       `json:"abi"`
	Runtime      string       `json:"runtime"`
	Capabilities []Capability `json:"capabilities"`
	EnvAllowlist []string     `json:"env_allowlist"`
	ImportList   []string     `json:"import_allowlist"`
	Size         int64        `json:"size"`
	ObjectKey    string       `json:"object_key"`
	State        State        `json:"state"`
	CreatedAt    time.Time    `json:"created_at"`
	PublishedAt  *time.Time   `json:"published_at,omitempty"`
	WithdrawnAt  *time.Time   `json:"withdrawn_at,omitempty"`
}

type RegisterRequest struct {
	TenantID     string       `json:"tenant_id"`
	Name         string       `json:"name"`
	Version      string       `json:"version"`
	Digest       string       `json:"digest"`
	Entry        string       `json:"entry"`
	ABI          string       `json:"abi"`
	Runtime      string       `json:"runtime"`
	Capabilities []Capability `json:"capabilities"`
	EnvAllowlist []string     `json:"env_allowlist"`
	ImportList   []string     `json:"import_allowlist"`
	ObjectKey    string       `json:"object_key"`
	Content      []byte       `json:"content"`
}

func Digest(content []byte) string {
	h := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(h[:])
}

func New(id string, in RegisterRequest, maxBytes int64, now time.Time) (Module, error) {
	if strings.TrimSpace(in.TenantID) == "" || strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Version) == "" {
		return Module{}, fmt.Errorf("tenant_id, name and version are required")
	}
	if len(in.Content) == 0 {
		return Module{}, fmt.Errorf("module content is required")
	}
	if int64(len(in.Content)) > maxBytes {
		return Module{}, fmt.Errorf("module size %d exceeds limit %d", len(in.Content), maxBytes)
	}
	actual := Digest(in.Content)
	if in.Digest != "" && in.Digest != actual {
		return Module{}, fmt.Errorf("%w: expected %s got %s", ErrInvalidDigest, in.Digest, actual)
	}
	if in.ABI == "" {
		in.ABI = "sandbox/v1"
	}
	if in.ABI != "sandbox/v1" {
		return Module{}, fmt.Errorf("%w: %s", ErrIncompatibleABI, in.ABI)
	}
	if in.Entry == "" {
		in.Entry = "run"
	}
	if in.Runtime == "" {
		in.Runtime = "sim/v1"
	}
	return Module{
		ID:           id,
		TenantID:     in.TenantID,
		Capabilities: in.Capabilities,
		Name:         in.Name,
		Version:      in.Version,
		Digest:       actual,
		Entry:        in.Entry,
		ABI:          in.ABI,
		Runtime:      in.Runtime,
		ImportList:   in.ImportList,
		EnvAllowlist: append([]string(nil), in.EnvAllowlist...),
		Size:         int64(len(in.Content)),
		ObjectKey:    in.ObjectKey,
		State:        StateDraft,
		CreatedAt:    now.UTC(),
	}, nil
}

func (m Module) Publish(now time.Time) (Module, error) {
	if m.State != StateDraft {
		return Module{}, fmt.Errorf("%w: cannot publish %s module", ErrInvalidTransition, m.State)
	}
	m.State = StatePublished
	t := now.UTC()
	m.PublishedAt = &t
	return m, nil
}
func (m Module) Withdraw(now time.Time) (Module, error) {
	if m.State != StatePublished {
		return Module{}, fmt.Errorf("%w: cannot withdraw %s module", ErrInvalidTransition, m.State)
	}
	m.State = StateWithdrawn
	t := now.UTC()
	m.WithdrawnAt = &t
	return m, nil
}
func (m Module) IsRunnable() bool { return m.State == StatePublished }
