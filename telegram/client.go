package telegram

import (
	"context"
	"log"
	"time"

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

// New creates a new Telegram client, handles authentication, and runs it in a background goroutine.
func New(appID int, appHash, phoneNumber string) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	client := telegram.NewClient(appID, appHash, telegram.Options{
		SessionStorage: &session.FileStorage{Path: "session_DO_NOT_SHARE.json"},
	})

	tgClient := &Client{
		client: client,
		ctx:    ctx,
		cancel: cancel,
	}

	passedAuthFlow := make(chan struct{})
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
			if duration, ok := tgerr.AsFloodWait(err); ok {
				log.Fatalf("Flood wait hit, cant signin for: %v\n", duration)
				return
			}

			log.Fatalf("Couldnt run client: %v\n", err)
		}
	}()

	<-passedAuthFlow
	return tgClient
}

// CreateChannel creates a new public channel with the given username.
func (c *Client) CreateChannel(username string) error {
	ctx, cancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer cancel()

	u, err := c.api.ChannelsCreateChannel(ctx, &tg.ChannelsCreateChannelRequest{Title: username, Broadcast: true})
	if err != nil {
		return err
	}

	// Handle different Updates types safely
	var channel *tg.Channel
	switch upd := u.(type) {
	case *tg.Updates:
		if len(upd.Chats) == 0 {
			return nil
		}
		if ch, ok := upd.Chats[0].(*tg.Channel); ok {
			channel = ch
		}
	default:
		// fallback: try to extract via type assertion to Updates
		if upd, ok := u.(*tg.Updates); ok && len(upd.Chats) > 0 {
			if ch, ok := upd.Chats[0].(*tg.Channel); ok {
				channel = ch
			}
		}
	}
	if channel == nil {
		return nil
	}
	inputChannel := &tg.InputChannel{
		ChannelID:  channel.GetID(),
		AccessHash: channel.AccessHash,
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
