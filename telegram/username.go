package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	FragmentBaseURL  = "https://fragment.com/"
	DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:141.0) Gecko/20100101 Firefox/141.0"
)

// Username statuses returned by CheckUsername.
const (
	StatusAvailable       = "Available"
	StatusTaken           = "Taken"
	StatusSold            = "Sold"
	StatusAuctioned       = "Auctioned or for sale"
	StatusRatelimit       = "Ratelimit"
	StatusUnknown         = "Unknown"
)

// httpClient is a var so tests can stub it.
var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

// CheckUsername queries fragment.com and returns a typed status.
// It returns an error for transport/parse failures; rate-limit is
// reported as StatusRatelimit with nil error so callers can back off
// instead of mistaking it for "Taken".
func CheckUsername(ctx context.Context, username string) (string, error) {
	status, err := getUser(ctx, username)
	if err != nil {
		return "", err
	}
	return status, nil
}

// IsUsernameAvailable reports whether a username is claimable.
// Network errors, rate-limits and unknown states return false
// (conservative: never claim on uncertain data).
func IsUsernameAvailable(username string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	status, err := getUser(ctx, username)
	if err != nil {
		return false
	}
	return status == StatusAvailable
}

func getUser(ctx context.Context, username string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", FragmentBaseURL+"username/"+username, nil)
	if err != nil {
		return "", err
	}
	setHeaders(req, username)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	return processResponse(resp)
}

func setHeaders(req *http.Request, username string) {
	referURL := fmt.Sprintf("%s?query=%s", FragmentBaseURL, username)

	req.Header.Set("User-Agent", DefaultUserAgent)
	req.Header.Set("X-Aj-Referer", referURL)
	req.Header.Set("Referer", referURL)
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Priority", "u=1")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("TE", "trailers")
}

func processResponse(resp *http.Response) (string, error) {
	if resp.StatusCode == http.StatusTooManyRequests {
		return StatusRatelimit, nil
	}
	// Limit body to 1 MiB to avoid OOM on unexpected payloads.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}

	if rData, ok := response["r"].(string); ok && rData == "/" {
		return StatusRatelimit, nil
	}
	// Fragment sometimes returns 429 via JSON without status code

	hData, ok := response["h"].(string)
	if !ok || strings.TrimSpace(hData) == "" {
		return StatusAvailable, nil
	}

	// More robust parsing: find status class
	// h contains like `<span class="tm-section-header-status tm-status-taken">`
	idx := strings.Index(hData, "tm-section-header-status")
	if idx == -1 {
		return StatusUnknown, nil
	}
	snippet := hData[idx:]
	// extract class that starts with tm-status-
	status := ""
	for _, cand := range []string{"tm-status-taken", "tm-status-avail", "tm-status-unavail", "tm-status-await"} {
		if strings.Contains(snippet, cand) {
			status = cand
			break
		}
	}
	if status == "" {
		// fallback to old split logic
		parts := strings.Split(snippet, "tm-section-header-status")
		if len(parts) > 1 {
			rest := strings.Split(parts[1], `">`)
			if len(rest) > 0 {
				status = strings.TrimSpace(rest[0])
			}
		}
	}

	statusMapping := map[string]string{
		"tm-status-taken":   StatusTaken,
		"tm-status-avail":   StatusAuctioned,
		"tm-status-unavail": StatusSold,
		"tm-status-await":   StatusTaken,
	}

	if mappedStatus, exists := statusMapping[status]; exists {
		return mappedStatus, nil
	}
	if strings.Contains(hData, "tm-status-taken") {
		return StatusTaken, nil
	}
	if strings.Contains(hData, "tm-status-avail") {
		return StatusAuctioned, nil
	}
	if strings.Contains(hData, "tm-status-unavail") {
		return StatusSold, nil
	}

	return StatusUnknown, nil
}
