package domain

import (
	"errors"
	"time"
)

type RevisionState string

const (
	RevisionProposed   RevisionState = "proposed"
	RevisionApplied    RevisionState = "applied"
	RevisionRolledBack RevisionState = "rolled_back"
)

var ErrRevisionTransition = errors.New("invalid quota revision transition")

type RevisionEvent struct {
	Kind string    `json:"kind"`
	At   time.Time `json:"at"`
}

type QuotaRevision struct {
	ID       string          `json:"id"`
	TenantID string          `json:"tenant_id"`
	Before   Quota           `json:"before"`
	After    Quota           `json:"after"`
	State    RevisionState   `json:"state"`
	Events   []RevisionEvent `json:"events"`
}

func NewQuotaRevision(id, tenantID string, before, after Quota, now time.Time) QuotaRevision {
	return QuotaRevision{
		ID: id, TenantID: tenantID, Before: before, After: after,
		State:  RevisionProposed,
		Events: []RevisionEvent{{Kind: string(RevisionProposed), At: now}},
	}
}

func (r *QuotaRevision) Apply(now time.Time) error {
	r.State = RevisionApplied
	r.Events = append(r.Events, RevisionEvent{Kind: string(RevisionApplied), At: now})
	return nil
}

func (r *QuotaRevision) Rollback(now time.Time) error {
	r.State = RevisionRolledBack
	r.Events = append(r.Events, RevisionEvent{Kind: string(RevisionRolledBack), At: now})
	return nil
}

func (r QuotaRevision) Snapshot() QuotaRevision {
	r.Events = append([]RevisionEvent(nil), r.Events...)
	return r
}
