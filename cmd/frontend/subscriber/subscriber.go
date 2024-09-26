package subscriber

import (
	"context"
	"fmt"
	"log"

	daprsdk "github.com/dapr/go-sdk/client"
	"golang.org/x/sync/errgroup"

	"github.com/lrascao/place/cmd/frontend/config"
	"github.com/lrascao/place/pkg/pixel"
)

func Start(ctx context.Context, client daprsdk.Client, cfg *config.Config, fn func(p pixel.Pixel) error) error {
	var g *errgroup.Group
	g, ctx = errgroup.WithContext(ctx)

	g.Go(func() error {
		subscription, err := client.Subscribe(ctx,
			daprsdk.SubscriptionOptions{
				PubsubName: cfg.PubSub.Name,
				Topic:      cfg.PubSub.Topic,
			})
		if err != nil {
			return fmt.Errorf("error subscribing to topic: %w", err)
		}
		// Close must always be called.
		defer subscription.Close()

		for {
			if ctx.Err() != nil {
				return nil
			}

			msg, err := subscription.Receive()
			if err != nil {
				fmt.Printf("error receiving message: %v", err)
			}
			if msg == nil {
				continue
			}

			// Process the event
			p := pixel.New()
			if err := p.Unmarshal(msg.RawData); err != nil {
				return fmt.Errorf("error unmarshaling pixel: %w", err)
			}
			log.Printf("received pixel event: %+v", p)

			if err := fn(p); err != nil {
				return fmt.Errorf("error handling pixel: %w", err)
			}

			// Acknowledge the message
			if err := msg.Success(); err != nil {
				fmt.Printf("error acknowledging message: %v", err)
			}
		}
	})

	return nil
}
