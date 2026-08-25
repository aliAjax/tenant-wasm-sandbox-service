package domain

import "context"

// ObjectStore retrieves immutable module artifacts from an external store.
// Implementations must return a copy or immutable view of the object bytes.
type ObjectStore interface {
	Put(context.Context, string, []byte) error
	Get(context.Context, string) ([]byte, error)
	Delete(context.Context, string) error
}

// SignatureVerifier establishes artifact provenance before registration.
type SignatureVerifier interface {
	Verify(context.Context, string, []byte, []byte) error
}

type Artifact struct {
	Key       string `json:"key"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
	MediaType string `json:"media_type"`
}
