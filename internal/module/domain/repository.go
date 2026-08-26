package domain

import (
	"context"
	"time"
)

type Repository interface {
	Create(context.Context, Module, []byte) error
	Update(context.Context, Module) error
	Get(context.Context, string) (Module, error)
	Content(context.Context, string) ([]byte, error)
	List(context.Context, string) ([]Module, error)
	FindVersion(context.Context, string, string, string) (Module, error)
}
type SignatureVerifier interface {
	Verify(context.Context, string, []byte, []byte) error
}
type Clock interface{ Now() time.Time }
type IDGenerator interface{ NewID(string) string }
