//go:build go1.21
// +build go1.21

package starform

import "context"

func afterFunc(ctx context.Context, f func()) (stop func() bool) {
	return context.AfterFunc(ctx, f)
}
