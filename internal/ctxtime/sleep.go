package ctxtime

import (
	"context"
	"time"
)

func Sleep(ctx context.Context, d time.Duration) error {
	if ctx == nil || d <= 0 {
		time.Sleep(d)
		return nil
	}

	t := time.NewTimer(d)
	select {
	case <-ctx.Done():
		t.Stop()
		return ctx.Err()
	case <-t.C:
	}
	return nil
}
