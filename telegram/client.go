package telegram

import (
	"context"
	"fmt"
	"time"

	"github.com/ixode0/TUS/config"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
)

type Client struct {
	client *telegram.Client
	api    *tg.Client
	ctx    context.Context
	cancel context.CancelFunc
}

// authTimeout bounds the whole sign-in wait so we never deadlock forever.
const authTimeout = 5 * time.Minute

// New creates a new Telegram client, handles authentication, and runs it in a background goroutine.
// It returns an error instead of calling log.Fatalf, and fails fast on
// missing/invalid credentials. Session is stored next to the config file
// (see config.SessionPath) so the CWD doesn't silently change identity.
func New(appID int, appHash, phoneNumber string) (*Client, error) {
	if appID == 0 || appHash == "" || phoneNumber == "" {
		return nil, fmt.Errorf("missing Telegram credentials: set api_id, api_hash and phone_number in config.json")
	}
	ctx, cancel := context.WithCancel(context.Background())
	sessionPath := config.SessionPath()
	client := telegram.NewClient(appID, appHash, telegram.Options{
		SessionStorage: &session.FileStorage{Path: sessionPath},
	})

	tgClient := &Client{
		client: client,
		ctx:    ctx,
		cancel: cancel,
	}

	passedAuthFlow := make(chan struct{})
	errCh := make(chan error, 1)
	authFlow := auth.NewFlow(SimpleAuthFlow{PhoneNumber: phoneNumber}, auth.SendCodeOptions{})

	go func() {
		err := client.Run(ctx, func(ctx context.Context) error {
			if err := client.Auth().IfNecessary(ctx, authFlow); err != nil {
				return err
			}

			tgClient.api = client.API()
			close(passedAuthFlow)

			<-ctx.Done()
			return ctx.Err()
		})

		if err != nil {
			// Don't log.Fatalf from a goroutine — report to the caller.
			select {
			case errCh <- err:
			default:
			}
		}
	}()

	select {
	case <-passedAuthFlow:
		return tgClient, nil
	case err := <-errCh:
		cancel()
		if duration, ok := tgerr.AsFloodWait(err); ok {
			return nil, fmt.Errorf("flood wait during sign-in, retry in %v: %w", duration, err)
		}
		return nil, fmt.Errorf("couldn't run client: %w", err)
	case <-time.After(authTimeout):
		cancel()
		return nil, fmt.Errorf("authentication timed out after %v (check phone/code/2FA input)", authTimeout)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// CreateChannel creates a new public channel with the given username.
// It returns an error when Telegram doesn't return a usable channel —
// never a silent nil that would look like a successful claim.
func (c *Client) CreateChannel(username string) error {
	ctx, cancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer cancel()

	u, err := c.api.ChannelsCreateChannel(ctx, &tg.ChannelsCreateChannelRequest{Title: username, Broadcast: true})
	if err != nil {
		return err
	}

	// Handle different Updates types safely.
	// Only *tg.Updates carries Chats; everything else is unexpected here.
	upd, ok := u.(*tg.Updates)
	if !ok {
		return fmt.Errorf("unexpected response creating channel %q: %T (claim NOT confirmed)", username, u)
	}
	if len(upd.Chats) == 0 {
		return fmt.Errorf("telegram returned no chats creating channel %q (claim NOT confirmed)", username)
	}
	ch, ok := upd.Chats[0].(*tg.Channel)
	if !ok {
		return fmt.Errorf("telegram returned %T instead of channel for %q (claim NOT confirmed)", upd.Chats[0], username)
	}
	if ch == nil {
		return fmt.Errorf("telegram returned nil channel for %q (claim NOT confirmed)", username)
	}
	inputChannel := &tg.InputChannel{
		ChannelID:  ch.GetID(),
		AccessHash: ch.AccessHash,
	}

	// Update the channel username.
	if _, err := c.api.ChannelsUpdateUsername(ctx, &tg.ChannelsUpdateUsernameRequest{
		Channel:  inputChannel,
		Username: username,
	}); err != nil {
		return err
	}

	return nil
}

// UpdateUsername updates the account username.
func (c *Client) UpdateUsername(newUsername string) error {
	ctx, cancel := context.WithTimeout(c.ctx, 15*time.Second)
	defer cancel()
	_, err := c.api.AccountUpdateUsername(ctx, newUsername)
	return err
}

// Close stops the client.
func (c *Client) Close() {
	if c.cancel != nil {
		c.cancel()
	}
}
