package subscriber

import (
	"context"
	"fmt"

	"github.com/dapr/go-sdk/service/common"
	daprcommon "github.com/dapr/go-sdk/service/common"
	daprd "github.com/dapr/go-sdk/service/grpc"

	"github.com/lrascao/place/cmd/frontend/config"
	"github.com/lrascao/place/pkg/pixel"
)

func Start(cfg *config.Config, fn func(p pixel.Pixel) error) error {
	// Dapr
	daprSrv, err := daprd.NewService(fmt.Sprintf(":%d", cfg.PubSub.Port))
	if err != nil {
		return fmt.Errorf("error creating dapr service: %w", err)
	}

	if err := daprSrv.AddTopicEventHandler(
		&daprcommon.Subscription{
			PubsubName: cfg.PubSub.Name,
			Topic:      cfg.PubSub.Topic,
		},
		func(ctx context.Context, e *common.TopicEvent) (bool, error) {
			p := pixel.New()
			if err := p.Unmarshal(e.RawData); err != nil {
				return false, fmt.Errorf("error unmarshaling pixel: %w", err)
			}

			if err := fn(p); err != nil {
				return false, fmt.Errorf("error handling pixel: %w", err)
			}

			return false, nil
		}); err != nil {
		return fmt.Errorf("error adding topic event handler: %w", err)
	}

	if err := daprSrv.AddHealthCheckHandler("health",
		func(context.Context) error {
			return nil
		}); err != nil {
		return fmt.Errorf("error adding health check handler: %w", err)
	}

	go func() error {
		if err := daprSrv.Start(); err != nil {
			return fmt.Errorf("error starting dapr service: %w", err)

		}
		return nil
	}()

	return nil
}
