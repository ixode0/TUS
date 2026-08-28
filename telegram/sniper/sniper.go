package sniper

import (
	"app/telegram"
	"fmt"
	"log"
	"time"

	"github.com/gotd/td/tgerr"
)

func ProcessAvailableUsernames(client *telegram.Client, claimMethod string, availableUsernamesChan <-chan string) {
	for username := range availableUsernamesChan {
		// handle each username in separate goroutine to not block other usernames
		go func(u string) {
			fmt.Printf("[%s] Found available username: %s -> start claiming every 1.5s\n", time.Now().Format(time.RFC3339), u)
			attempt := 0
			for {
				attempt++
				err := claimUsername(client, claimMethod, u)
				if err == nil {
					fmt.Printf("[%s] Successfully claimed: %s via %s (attempt %d)\n", time.Now().Format(time.RFC3339), u, claimMethod, attempt)
					return
				}
				// if username already occupied by someone else faster, stop retrying
				if tgerr.Is(err, "USERNAME_OCCUPIED") || tgerr.Is(err, "USERNAME_NOT_AVAILABLE") {
					log.Printf("[%s] %s not available anymore (attempt %d): %v -> stop retrying", time.Now().Format(time.RFC3339), u, attempt, err)
					return
				}
				if d, ok := tgerr.AsFloodWait(err); ok {
					log.Printf("[%s] FloodWait for %s: %v (attempt %d): %v -> waiting flood", time.Now().Format(time.RFC3339), u, d, attempt, err)
					time.Sleep(d)
					continue
				}
				log.Printf("[%s] Failed to claim %s (attempt %d): %v -> retry in 1.5s", time.Now().Format(time.RFC3339), u, attempt, err)
				time.Sleep(1500 * time.Millisecond)
			}
		}(username)
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
