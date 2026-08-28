package main

import (
	"app/config"
	"app/telegram"
	"app/telegram/monitor"
	"app/telegram/sniper"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

var (
	cfg = config.GetConfig()
	wg  sync.WaitGroup
)

func main() {
	phoneNumber := cfg.Telegram.PhoneNumber
	if !strings.HasPrefix(phoneNumber, "+") {
		log.Fatal("Phone number must be in international format (e.g, +12848428429)")
	}

	claimMethod := cfg.ClaimTo
	if claimMethod != "user" && claimMethod != "channel" {
		log.Fatal("The value of 'claim_to' must be either 'user' or 'channel'")
	}

	usernames := cfg.Usernames
	if len(usernames) == 0 {
		log.Fatal("Please provide at least 1 username")
	}
	if cfg.CheckSleepTimeMS < 50 {
		log.Println("Warning: sleep_between_check < 50ms may trigger rate limits")
	}

	client := telegram.New(cfg.Telegram.APIID, cfg.Telegram.APIHash, phoneNumber)
	defer client.Close()

	availableUsernamesChan := make(chan string, 10)

	fmt.Printf("Monitoring %v username(s): %v (interval %dms, claim_to=%s)\n", len(usernames), usernames, cfg.CheckSleepTimeMS, claimMethod)
	wg.Add(1)
	go monitor.StartMonitor(usernames, cfg.CheckSleepTimeMS, availableUsernamesChan, &wg)
	go sniper.ProcessAvailableUsernames(client, claimMethod, availableUsernamesChan)

	// Graceful shutdown on SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	fmt.Println("\nShutting down...")
	close(availableUsernamesChan)
	wg.Wait()
}
