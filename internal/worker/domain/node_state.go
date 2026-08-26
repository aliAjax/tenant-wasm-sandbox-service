package domain

import "time"

func (n *Node) ApplyTransition(next NodeState, reason string, now time.Time) error {
	n.State = next
	n.Reason = reason
	n.UpdatedAt = now
	return nil
}
