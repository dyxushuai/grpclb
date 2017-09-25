package grpclb

import "golang.org/x/net/context"

// Verifier check if the hasher is hit.
type Verifier interface {
	Verify(uint32) bool
	Wait(context.Context, uint32) error
}
