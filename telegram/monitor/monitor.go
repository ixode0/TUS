package monitor

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/ixode0/TUS/telegram"
)

// maxConcurrentChecks bounds parallel fragment polls (B4): sequential O(n)
// scans lose the race on 10+ names; unbounded parallelism hammers fragment.
const maxConcurrentChecks = 5

// heartbeatInterval: how often to log waiting progress so the user sees
// the monitor is alive ("проверяю... рейт-лимитов нет").
const heartbeatInterval = 30 * time.Second

type checkResult struct {
	index    int
	username string
	status   string
	err      error
}

// StartMonitor polls fragment.com for each username until ctx is cancelled
// or all usernames are claimed. It owns a private copy of the list, so there
// is no aliasing/mutex issue with the caller's slice.
// Ratelimit/unknown states back off instead of being treated as "Taken".
// Each scan round checks pending usernames concurrently (bounded), with a
// per-check timeout; sends/removals stay sequential to avoid races.
func StartMonitor(ctx context.Context, usernames []string, sleepTimeMs int, availableUsernamesChan chan<- string, wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}
	// Private copy — no shared backing array with the caller.
	pending := append([]string(nil), usernames...)

	backoff := time.Duration(sleepTimeMs) * time.Millisecond
	const maxBackoff = 30 * time.Second
	lastHeartbeat := time.Now()

	for len(pending) > 0 {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// One scan round: check all pending concurrently (bounded), then
		// process results sequentially. Exactly ONE sleep per round:
		// per-username sleeps multiply the round by N, and per-username
		// backoff doubling explodes (2^N). Ratelimit backoff is global.
		results := checkAll(ctx, pending)

		sawRatelimit := false
		for _, r := range results {
			select {
			case <-ctx.Done():
				return
			default:
			}

			username := r.username

			if r.err != nil {
				log.Printf("monitor: check %s failed: %v (retrying)", username, r.err)
				continue
			}

			switch r.status {
			case telegram.StatusAvailable:
				// Non-blocking send respecting shutdown.
				select {
				case availableUsernamesChan <- username:
					pending = removeUsername(pending, username)
					backoff = time.Duration(sleepTimeMs) * time.Millisecond
				case <-ctx.Done():
					return
				}
			case telegram.StatusRatelimit:
				sawRatelimit = true
			case telegram.StatusUnknown:
				log.Printf("monitor: unknown status for %s (fragment layout may have changed)", username)
			default: // Taken / Sold / Auctioned — keep watching
			}
		}

		if len(pending) == 0 {
			return
		}

		// Single round sleep: global backoff when any ratelimit was seen.
		if sawRatelimit {
			log.Printf("monitor: fragment ratelimit hit, backing off %v", backoff)
			if !sleepCtx(ctx, backoff) {
				return
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		} else {
			if !sleepCtx(ctx, time.Duration(sleepTimeMs)*time.Millisecond) {
				return
			}
			// Waiting progress: prove we're alive when all is quiet.
			if time.Since(lastHeartbeat) >= heartbeatInterval {
				log.Printf("monitor: проверяю %v... рейт-лимитов нет", pending)
				lastHeartbeat = time.Now()
			}
		}
	}
}

// checkAll polls every username concurrently, bounded by
// maxConcurrentChecks. Each check has its own 15s timeout. Results keep
// input order so processing stays deterministic.
func checkAll(ctx context.Context, pending []string) []checkResult {
	results := make([]checkResult, len(pending))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentChecks)
	for i, username := range pending {
		wg.Add(1)
		go func(i int, username string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[i] = checkResult{index: i, username: username, err: ctx.Err()}
				return
			}
			cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			status, err := telegram.CheckUsername(cctx, username)
			results[i] = checkResult{index: i, username: username, status: status, err: err}
		}(i, username)
	}
	wg.Wait()
	return results
}

func removeUsername(pending []string, username string) []string {
	for i, u := range pending {
		if u == username {
			return append(pending[:i], pending[i+1:]...)
		}
	}
	return pending
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
