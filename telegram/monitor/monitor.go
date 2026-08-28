package monitor

import (
	"app/telegram"
	"sync"
	"time"
)

var mu sync.Mutex

func StartMonitor(usernames []string, sleepTimeMs int, availableUsernamesChan chan<- string, wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}
	for {
		checkUsernames(&usernames, sleepTimeMs, availableUsernamesChan)
	}
}

func checkUsernames(usernames *[]string, sleepTimeMs int, availableUsernamesChan chan<- string) {
	for i := 0; i < len(*usernames); i++ {
		username := (*usernames)[i]
		if telegram.IsUsernameAvailable(username) {
			availableUsernamesChan <- username

			mu.Lock()
			*usernames = append((*usernames)[:i], (*usernames)[i+1:]...)
			mu.Unlock()
			i-- // adjust index after removal
			continue
		}

		time.Sleep(time.Duration(sleepTimeMs) * time.Millisecond)
	}
	// small pause between full scans to avoid tight loop when list empty
	if len(*usernames) == 0 {
		time.Sleep(time.Duration(sleepTimeMs) * time.Millisecond)
	}
}
