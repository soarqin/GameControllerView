//go:build !windows

package gamepad

import "context"

// Run blocks until ctx is cancelled.
// Gamepad reading is not yet implemented on non-Windows platforms.
func (r *Reader) Run(ctx context.Context) {
	<-ctx.Done()
	close(r.changes)
}
