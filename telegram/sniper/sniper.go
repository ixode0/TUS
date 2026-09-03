package sniper

import (
	"context"
	"fmt"
	"log"
	"math/rand"
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
	fmt.Printf("[%s] Found available username: %s -> start claiming every 1.5s\n", time.Now().Format(time.RFC3339), u)
	attempt := 0
	for {
		select {
		case <-ctx.Done():
			log.Printf("[%s] %s: shutdown, stopping claim loop after %d attempts", time.Now().Format(time.RFC3339), u, attempt)
			return
		default:
		}

		attempt++
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
			// Add 1-3s jitter on top of the required wait to avoid
			// re-hitting FloodWait on the exact second.
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
		log.Printf("[%s] Failed to claim %s (attempt %d): %v -> retry in 1.5s", time.Now().Format(time.RFC3339), u, attempt, err)
		if !sleepCtx(ctx, 1500*time.Millisecond) {
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
