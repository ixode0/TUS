package monitor

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/ixode0/TUS/telegram"
)

// StartMonitor polls fragment.com for each username until ctx is cancelled
// or all usernames are claimed. It owns a private copy of the list, so there
// is no aliasing/mutex issue with the caller's slice.
// Ratelimit/unknown states back off instead of being treated as "Taken".
func StartMonitor(ctx context.Context, usernames []string, sleepTimeMs int, availableUsernamesChan chan<- string, wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}
	// Private copy — no shared backing array with the caller.
	pending := append([]string(nil), usernames...)

	backoff := time.Duration(sleepTimeMs) * time.Millisecond
	const maxBackoff = 30 * time.Second

	for len(pending) > 0 {
		select {
		case <-ctx.Done():
			return
		default:
		}

		for i := 0; i < len(pending); i++ {
			select {
			case <-ctx.Done():
				return
			default:
			}

			username := pending[i]
			cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
			status, err := telegram.CheckUsername(cctx, username)
			cancel()

			if err != nil {
				log.Printf("monitor: check %s failed: %v (retrying)", username, err)
				if !sleepCtx(ctx, time.Duration(sleepTimeMs)*time.Millisecond) {
					return
				}
				continue
			}

			switch status {
			case telegram.StatusAvailable:
				// Non-blocking send respecting shutdown.
				select {
				case availableUsernamesChan <- username:
					pending = append(pending[:i], pending[i+1:]...)
					i--
					backoff = time.Duration(sleepTimeMs) * time.Millisecond
				case <-ctx.Done():
					return
				}
			case telegram.StatusRatelimit:
				log.Printf("monitor: fragment ratelimit hit, backing off %v", backoff)
				if !sleepCtx(ctx, backoff) {
					return
				}
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				// Don't advance extra; retry same username after backoff.
				i--
			case telegram.StatusUnknown:
				log.Printf("monitor: unknown status for %s (fragment layout may have changed)", username)
				if !sleepCtx(ctx, time.Duration(sleepTimeMs)*time.Millisecond) {
					return
				}
			default: // Taken / Sold / Auctioned — keep watching
				if !sleepCtx(ctx, time.Duration(sleepTimeMs)*time.Millisecond) {
					return
				}
			}
		}

		if len(pending) == 0 {
			return
		}
	}
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
