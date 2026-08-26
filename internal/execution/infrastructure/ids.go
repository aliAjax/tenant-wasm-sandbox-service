package infrastructure

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

type RandomIDs struct{ counter atomic.Uint64 }

func (r *RandomIDs) NewID(prefix string) string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err == nil {
		return fmt.Sprintf("%s_%d_%s", prefix, time.Now().UnixMilli(), hex.EncodeToString(b[:]))
	}
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixMilli(), r.counter.Add(1))
}
