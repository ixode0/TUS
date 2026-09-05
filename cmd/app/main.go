package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/ixode0/TUS/config"
	"github.com/ixode0/TUS/telegram"
	"github.com/ixode0/TUS/telegram/monitor"
	"github.com/ixode0/TUS/telegram/sniper"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg := config.GetConfig()
	if cfg.CheckSleepTimeMS < config.MinSafeSleepMS {
		log.Printf("Warning: sleep_between_check=%dms < %dms may trigger rate limits (FloodWait); recommended %dms",
			cfg.CheckSleepTimeMS, config.MinSafeSleepMS, config.DefaultSleepMS)
	}

	client, err := telegram.New(cfg.Telegram.APIID, cfg.Telegram.APIHash, cfg.Telegram.PhoneNumber)
	if err != nil {
		return fmt.Errorf("telegram client: %w", err)
	}
	defer client.Close()

	// Cancellable context for graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	availableUsernamesChan := make(chan string, 10)

	fmt.Printf("Monitoring %v username(s): %v (interval %dms, claim_to=%s)\n", len(cfg.Usernames), cfg.Usernames, cfg.CheckSleepTimeMS, cfg.ClaimTo)

	var wg sync.WaitGroup
	// Monitor owns one Done; each claim loop adds its own.
	wg.Add(1)
	go monitor.StartMonitor(ctx, cfg.Usernames, cfg.CheckSleepTimeMS, availableUsernamesChan, &wg)
	go sniper.ProcessAvailableUsernames(ctx, client, cfg.ClaimTo, availableUsernamesChan, &wg)

	<-ctx.Done()
	fmt.Println("\nShutting down... (waiting for in-flight claims)")
	// Give claim loops a moment to notice ctx cancellation, then wait.
	// Closing the channel is NOT needed: monitor exits on ctx, sniper exits
	// on ctx or closed channel — no send-on-closed panic possible.
	stop()
	wg.Wait()
	fmt.Println("Bye.")
	return nil
}
