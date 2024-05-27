package subscriber

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/dapr/go-sdk/service/common"
	daprcommon "github.com/dapr/go-sdk/service/common"
	daprd "github.com/dapr/go-sdk/service/grpc"
	"github.com/diagridio/catalyst-go/pkg/tunnels"
	"github.com/diagridio/diagrid-cloud-go/cloudruntime"
	"github.com/spf13/viper"

	"github.com/lrascao/place/cmd/frontend/config"
	"github.com/lrascao/place/pkg/pixel"
	"github.com/lrascao/stacktrace"
)

func Start(ctx context.Context, cfg *config.Config, fn func(p pixel.Pixel) error) error {
	// create the catalyst client
	opts := []cloudruntime.CloudruntimeClientOption{
		cloudruntime.WithAPIKeyToken(viper.GetString("diagrid_api_key")),
	}
	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}
	catalyst, err := cloudruntime.NewCloudruntimeClient(httpClient, cfg.Diagrid.Endpoint, opts...)
	if err != nil {
		return stacktrace.Propagate(err)
	}

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

	// create an app tunnel
	conn, err := catalyst.ConnectAppTunnel(ctx, cfg.Diagrid.Project.Name, cfg.Diagrid.Project.Frontend, "")
	if err != nil {
		return stacktrace.Propagate(err)
	}

	tunnelReady := make(chan bool)
	go func() {
		if err := tunnels.ListenBlocking(ctx, tunnelReady,
			cfg.Diagrid.OrganizationID,
			cfg.Diagrid.Project.Name,
			cfg.Diagrid.Project.Frontend,
			fmt.Sprintf("%d", cfg.PubSub.Port),
			conn); err != nil {
			fmt.Printf("error listening on tunnel: %v", err)
		}
	}()
	// wait for app tunnel to be ready
	<-tunnelReady

	go func() error {
		if err := daprSrv.Start(); err != nil {
			return fmt.Errorf("error starting dapr service: %w", err)
		}
		return nil
	}()

	return nil
}
