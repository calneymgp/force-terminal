// SPDX-License-Identifier: Apache-2.0
package filestore

import (
	"context"
	"time"
)

// FlushForUpdate waits for an in-flight periodic flush, then performs its own
// flush. Returning nil acknowledges durable writes, never merely a busy cache.
func (s *FileStore) FlushForUpdate(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, err := s.FlushCache(ctx)
		if err != errFlushInProgress {
			return err
		}
		timer := time.NewTimer(20 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
