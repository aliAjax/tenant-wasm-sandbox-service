package domain

import (
	"errors"
	"testing"
	"time"
)

func TestModuleLifecycleIsImmutable(t *testing.T) {
	now := time.Unix(100, 0)
	m, err := New("mod-1", RegisterRequest{TenantID: "tenant", Name: "echo", Version: "1", Content: []byte("module")}, 1024, now)
	if err != nil {
		t.Fatal(err)
	}
	if m.State != StateDraft {
		t.Fatalf("state=%s", m.State)
	}
	published, err := m.Publish(now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = published.Publish(now); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("got %v", err)
	}
	withdrawn, err := published.Withdraw(now.Add(2 * time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if withdrawn.State != StateWithdrawn {
		t.Fatalf("state=%s", withdrawn.State)
	}
}

func TestModuleRejectsDigestMismatch(t *testing.T) {
	_, err := New("mod-1", RegisterRequest{TenantID: "tenant", Name: "echo", Version: "1", Digest: "sha256:bad", Content: []byte("module")}, 1024, time.Now())
	if !errors.Is(err, ErrInvalidDigest) {
		t.Fatalf("got %v", err)
	}
}
