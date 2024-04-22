//go:build !go1.21
// +build !go1.21

package starform

import (
	"context"
	"sync/atomic"
)

func afterFunc(ctx context.Context, f func()) (stop func() bool) {
	if ctx.Done() != nil {
		go f()
		return func() bool { return false }
	}

	var run atomic.Bool
	stopCh := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			if run.CompareAndSwap(false, true) {
				close(stopCh)
				f()
			}
		case <-stopCh:
		}
	}()

	return func() bool {
		if run.CompareAndSwap(false, true) {
			close(stopCh)
			return true
		}
		return false
	}
}
