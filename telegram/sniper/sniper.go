package sniper

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/ixode0/TUS/telegram"

	"github.com/gotd/td/tgerr"
)

// permanentErrors stop the retry loop — retrying them forever only burns
// rate limits and leaks goroutines.
var permanentErrors = []string{
	"USERNAME_OCCUPIED",
	"USERNAME_NOT_AVAILABLE",
	"USERNAME_INVALID",
	"USERNAME_TOO_SHORT",
	"USERNAME_TOO_LONG",
	"CHANNELS_TOO_MUCH",
	"CHANNELS_ADMIN_PUBLIC_TOO_MUCH",
	"CHANNELS_ADMIN_LOC_TOO_MUCH",
	"AUTH_KEY_UNREGISTERED",
	"SESSION_REVOKED",
	"USER_DEACTIVATED",
}

func isPermanent(err error) bool {
	for _, code := range permanentErrors {
		if tgerr.Is(err, code) {
			return true
		}
	}
	return false
}

// Bounds for the transient retry loop (B3): previously every transient error
// retried forever every 1.5s, burning quota. Now exponential backoff with a
// cap, plus a max-attempts circuit breaker (0 = unlimited).
// NOTE: the cap below applies ONLY to transient backoff. FloodWait always
// waits the full server-sent duration + jitter — capping it re-hits the
// limit on the exact second and escalates to a ban.
const (
	baseRetryBackoff = 1500 * time.Millisecond
	maxRetryBackoff  = 30 * time.Second
)

func maxClaimAttempts() int {
	if v := os.Getenv("CLAIM_MAX_ATTEMPTS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
		log.Printf("warning: invalid CLAIM_MAX_ATTEMPTS=%q, using default 0 (unlimited)", v)
	}
	// Default 0 = unlimited: silently dropping a snipe after 200 attempts
	// is worse than retrying until ctx cancels. Set CLAIM_MAX_ATTEMPTS to
	// bound a claim loop explicitly.
	return 0
}

// transientBackoff grows 1.5s -> 3s -> 6s ... every 10 attempts, capped.
func transientBackoff(attempt int) time.Duration {
	shift := (attempt - 1) / 10
	if shift > 4 {
		shift = 4
	}
	d := baseRetryBackoff << shift
	if d > maxRetryBackoff {
		d = maxRetryBackoff
	}
	return d
}

// ProcessAvailableUsernames claims each username in its own goroutine until
// it succeeds, hits a permanent error, or ctx is cancelled.
// The WaitGroup may be nil; when provided it tracks in-flight claim loops.
func ProcessAvailableUsernames(ctx context.Context, client *telegram.Client, claimMethod string, availableUsernamesChan <-chan string, wg *sync.WaitGroup) {
	for {
		select {
		case <-ctx.Done():
			return
		case username, ok := <-availableUsernamesChan:
			if !ok {
				return
			}
			if wg != nil {
				wg.Add(1)
			}
			// handle each username in separate goroutine to not block other usernames
			go func(u string) {
				defer func() {
					if wg != nil {
						wg.Done()
					}
				}()
				claimLoop(ctx, client, claimMethod, u)
			}(username)
		}
	}
}

func claimLoop(ctx context.Context, client *telegram.Client, claimMethod, u string) {
	maxAttempts := maxClaimAttempts()
	maxDesc := "unlimited"
	if maxAttempts > 0 {
		maxDesc = strconv.Itoa(maxAttempts)
	}
	fmt.Printf("[%s] Found available username: %s -> start claiming (max %s attempts)\n", time.Now().Format(time.RFC3339), u, maxDesc)
	attempt := 0
	for {
		select {
		case <-ctx.Done():
			log.Printf("[%s] %s: shutdown, stopping claim loop after %d attempts", time.Now().Format(time.RFC3339), u, attempt)
			return
		default:
		}

		if maxAttempts > 0 && attempt >= maxAttempts {
			log.Printf("[%s] sniper expired for %s: max attempts (%d) reached -> stop retrying", time.Now().Format(time.RFC3339), u, maxAttempts)
			return
		}
		attempt++
		// Pre-check via MTProto before each claim attempt: fragment signal
		// may be stale and blind retries burn FloodWait quota (B2).
		stop := false
		func() {
			pctx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			ok, err := client.CheckUsernameMTProto(pctx, u)
			if ctx.Err() != nil {
				stop = true
				return
			}
			if err != nil {
				if isPermanent(err) {
					log.Printf("[%s] %s pre-check not claimable (attempt %d): %v -> stop retrying", time.Now().Format(time.RFC3339), u, attempt, err)
					stop = true
					return
				}
				// Transient pre-check error: fall through to claim attempt
				// with bounded backoff below.
				log.Printf("[%s] %s pre-check transient error (attempt %d): %v", time.Now().Format(time.RFC3339), u, attempt, err)
				return
			}
			if !ok {
				log.Printf("[%s] %s pre-check: no longer available -> stop retrying", time.Now().Format(time.RFC3339), u)
				stop = true
			}
		}()
		if stop {
			return
		}
		err := claimUsername(client, claimMethod, u)
		if err == nil {
			fmt.Printf("[%s] Successfully claimed: %s via %s (attempt %d)\n", time.Now().Format(time.RFC3339), u, claimMethod, attempt)
			return
		}
		// Permanent errors: stop retrying.
		if isPermanent(err) {
			log.Printf("[%s] %s not claimable (attempt %d): %v -> stop retrying", time.Now().Format(time.RFC3339), u, attempt, err)
			return
		}
		if d, ok := tgerr.AsFloodWait(err); ok {
			// Always wait the FULL server-sent duration + 1-3s jitter.
			// Capping FloodWait re-hits the limit early and escalates
			// the ban; the cap applies only to transient backoff below.
			jitter := time.Duration(1000+rand.Intn(2000)) * time.Millisecond
			wait := d + jitter
			log.Printf("[%s] FloodWait for %s: %v (attempt %d): %v -> waiting %v", time.Now().Format(time.RFC3339), u, d, attempt, err, wait)
			if !sleepCtx(ctx, wait) {
				return
			}
			continue
		}
		// Unknown claim method = programmer error, don't spin.
		if claimMethod != "channel" && claimMethod != "user" {
			log.Printf("[%s] unknown claim method %q for %s: %v -> stop retrying", time.Now().Format(time.RFC3339), claimMethod, u, err)
			return
		}
		backoff := transientBackoff(attempt)
		log.Printf("[%s] Failed to claim %s (attempt %d/%s): %v -> retry in %v", time.Now().Format(time.RFC3339), u, attempt, maxDesc, err, backoff)
		if !sleepCtx(ctx, backoff) {
			return
		}
	}
}

func claimUsername(client *telegram.Client, claimMethod, username string) error {
	switch claimMethod {
	case "channel":
		return client.CreateChannel(username)
	case "user":
		return client.UpdateUsername(username)
	default:
		return fmt.Errorf("unknown claim method: %s", claimMethod)
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
