package domain

import "context"

type Repository interface {
	Create(context.Context, Execution) error
	Update(context.Context, Execution) error
	Get(context.Context, string) (Execution, error)
	List(context.Context, string, int) ([]Execution, error)
	FindIdempotency(context.Context, string, string) (Execution, bool)
	SaveIdempotency(context.Context, string, string, string)
}
type Scheduler interface {
	Submit(context.Context, string, string) error
	Cancel(string) bool
}
type Meter interface {
	Record(context.Context, Execution) error
}
type Callback interface {
	Deliver(context.Context, Execution) error
}
