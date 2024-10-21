package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	daprsdk "github.com/dapr/go-sdk/client"
	"github.com/gorilla/websocket"

	"github.com/lrascao/place/cmd/frontend/config"
	"github.com/lrascao/place/pkg/pixel"
	"github.com/lrascao/place/pkg/user"
)

type Client interface {
	Handle(context.Context) error
	Send(any) error
}

type client struct {
	conn *websocket.Conn
	dapr daprsdk.Client
	cfg  config.Config
}

var ErrPixelNotFound = fmt.Errorf("pixel not found")

func New(conn *websocket.Conn, dapr daprsdk.Client, cfg config.Config) Client {
	return &client{
		conn: conn,
		dapr: dapr,
		cfg:  cfg,
	}
}

func (c *client) Handle(ctx context.Context) error {
	var req Request

	// First message from the client is their info
	if err := c.conn.ReadJSON(&req); err != nil {
		return fmt.Errorf("error reading user info message: %w", err)
	}
	// assert that it the case
	if req.Type != RequestTypeUserInfo {
		return fmt.Errorf("expected user info, got %v", req.Type)
	}

	// create the user
	u := user.New()
	if err := u.Unmarshal([]byte(req.Data)); err != nil {
		return fmt.Errorf("error unmarshalling user: %w", err)
	}
	slog.Debug("received user info: ", u)

	// Goroutine to handle context cancellation and close the WebSocket
	go func() {
		<-ctx.Done()
		slog.Info("Closing WebSocket connection for client", "remoteaddr", c.conn.RemoteAddr())
		if err := c.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Server shutting down")); err != nil {
			slog.Error("Error sending close message: %v", err)
		}
		c.conn.Close()
	}()

	// go into a loop of handling client messages
	for {
		// Listen for incoming messages or context cancellation
		select {
		case <-ctx.Done():
			return nil // Exit when the context is canceled
		default:
			// Read the message from the client
			if err := c.conn.ReadJSON(&req); err != nil {
				return fmt.Errorf("error reading request: %w", err)
			}
		}

		switch req.Type {
		case RequestTypePut:
			p := pixel.New()
			if err := p.Unmarshal([]byte(req.Data)); err != nil {
				slog.Error("error unmarshalling pixel: ", err)
				break
			}
			slog.Info("received put pixel request", "pixel", p)

			data := PixelMetadata{
				Pixel: p,
				User:  u,
			}
			if err := c.savePixel(ctx, u.Name, data); err != nil {
				slog.Error("error saving pixel: ", err)
				break
			}

			// broadcast event to all clients
			if err := c.broadcast(ctx, data); err != nil {
				slog.Error("error broadcasting pixel: ", err)
				break
			}

		case RequestTypePixelInfo:
			p := pixel.New()
			if err := p.Unmarshal([]byte(req.Data)); err != nil {
				slog.Error("error unmarshalling pixel: ", err)
				break
			}
			slog.Info("received pixel info request", "pixel", p)

			// fetch the pixel data, encode it into json and send it back
			data, err := c.getPixel(ctx, p)
			if err != nil && err != ErrPixelNotFound {
				slog.Error("Error getting pixel info: ", err)
				break
			}
			if err == ErrPixelNotFound {
				break
			}

			jsonData, err := json.Marshal(data)
			if err != nil {
				slog.Error("Error marshalling pixel info: ", err)
				break
			}

			// send back the reply
			if err := c.Send(Event{
				Type: EventTypePixelInfo,
				Data: string(jsonData),
			}); err != nil {
				slog.Error("error sending pixel info: ", err)
				break
			}

		case RequestTypeCanvas:
			slog.Info("received canvas request")

			// get the canvas
			canvas, err := c.getCanvas(ctx)
			if err != nil {
				slog.Error("Error getting canvas: ", err)
				break
			}

			jsonData, err := json.Marshal(canvas)
			if err != nil {
				slog.Error("Error marshalling canvas: ", err)
				break
			}

			// send back the reply
			if err := c.Send(Event{
				Type: EventTypeCanvas,
				Data: string(jsonData),
			}); err != nil {
				slog.Error("error sending canvas: ", err)
				break
			}

		case RequestTypeCooldown:
			slog.Info("received cooldown request")

			// fetch the cooldown value
			cooldown, err := c.getCooldown(ctx, u.Name)
			if err != nil {
				slog.Error("Error getting cooldown: ", err)
				break
			}

			jsonData, err := json.Marshal(cooldown)
			if err != nil {
				slog.Error("Error marshalling cooldown: ", err)
				break
			}

			// send back the reply
			if err := c.Send(Event{
				Type: EventTypeCooldown,
				Data: string(jsonData),
			}); err != nil {
				slog.Error("error sending cooldown: ", err)
				break
			}

		default:
			slog.Warn("unknown client request: ", req)
		}
	}
}

func (c *client) Send(data any) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error marshalling data: %w", err)
	}

	if err := c.conn.WriteMessage(websocket.TextMessage, jsonData); err != nil {
		return fmt.Errorf("error sending data: %w", err)
	}

	return nil
}

func (c *client) savePixel(ctx context.Context, username string, data PixelMetadata) error {
	// are we in cooldown?
	cooldown, err := c.getCooldown(ctx, username)
	if err != nil {
		return fmt.Errorf("error getting user's cooldown: %w", err)
	}
	if cooldown > 0 {
		return fmt.Errorf("user is in cooldown: %d", cooldown)
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error marshalling pixel data: %w", err)
	}

	cooldownKey := cooldownUserKey(username)
	cooldownJsonData, err := json.Marshal(c.cfg.Cooldown.TTL)
	if err != nil {
		return fmt.Errorf("error marshalling cooldown data: %w", err)
	}

	metadata := map[string]string{"ttlInSeconds": c.cfg.Cooldown.TTL}
	if err := c.dapr.SaveState(ctx, c.cfg.Cooldown.Name, cooldownKey, cooldownJsonData, metadata); err != nil {
		return fmt.Errorf("error saving pixel data: %w", err)
	}

	key := fmt.Sprintf("c%d_%d", data.GetX(), data.GetY())
	if err := c.dapr.SaveState(ctx, c.cfg.StateStore.Name, key, jsonData, nil); err != nil {
		return fmt.Errorf("error saving pixel data: %w", err)
	}

	return nil
}

func (c *client) getPixel(ctx context.Context, p pixel.Pixel) (PixelMetadata, error) {
	key := fmt.Sprintf("c%d_%d", p.GetX(), p.GetY())
	item, err := c.dapr.GetState(ctx, c.cfg.StateStore.Name, key, nil)
	if err != nil {
		return PixelMetadata{}, fmt.Errorf("error getting pixel data: %w", err)
	}
	if item.Value == nil {
		return PixelMetadata{}, ErrPixelNotFound
	}

	var meta PixelMetadata
	if err := json.Unmarshal(item.Value, &meta); err != nil {
		return PixelMetadata{}, fmt.Errorf("error unmarshalling pixel data: %w", err)
	}

	return meta, nil
}

func (c *client) broadcast(ctx context.Context, data PixelMetadata) error {
	jsonData, err := json.Marshal(data.Pixel)
	if err != nil {
		return fmt.Errorf("error marshalling pixel data: %w", err)
	}

	if err := c.dapr.PublishEvent(ctx,
		c.cfg.PubSub.Name,
		c.cfg.PubSub.Topic,
		jsonData); err != nil {
		return fmt.Errorf("error broadcasting pixel data: %w", err)
	}

	return nil
}

func (c *client) getCanvas(ctx context.Context) ([]pixel.Pixel, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	query := "{}"
	resp, err := c.dapr.QueryStateAlpha1(ctx, c.cfg.StateStore.Name, query, nil)
	if err != nil {
		return nil, fmt.Errorf("error getting pixel data: %w", err)
	}

	var pixels []pixel.Pixel
	for _, item := range resp.Results {
		// ----------- SNIP --------------------
		// postgresql v1 QueryStateAlpha1 op returns base64 encoded values
		// drop this when it gets fixed
		vs, err := strconv.Unquote(string(item.Value))
		if err != nil {
			slog.Error("error unquoting value", "err", err)
			continue
		}
		v, err := base64.StdEncoding.DecodeString(vs)
		if err != nil {
			slog.Error("error decoding base64 value", "err", err)
			continue
		}
		// -------------------------------------
		var m PixelMetadata
		if err := json.Unmarshal(v, &m); err != nil {
			slog.Error("error unmarshalling pixel data [%v]: %v", string(v), err)
			continue
		}

		pixels = append(pixels, m.Pixel)
	}

	return pixels, nil
}

func (c *client) getCooldown(ctx context.Context, username string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	key := cooldownUserKey(username)
	resp, err := c.dapr.GetState(ctx, c.cfg.Cooldown.Name, key, nil)
	if err != nil {
		return 0, fmt.Errorf("error getting user's cooldown: %w", err)
	}
	if resp.Value == nil {
		return 0, nil
	}

	var strCooldown string
	if err := json.Unmarshal(resp.Value, &strCooldown); err != nil {
		return 0, fmt.Errorf("error unmarshalling user's cooldown: %w", err)
	}

	cooldown, err := strconv.Atoi(strCooldown)
	if err != nil {
		return 0, fmt.Errorf("error converting user's cooldown to integer (%s): %w",
			strCooldown, err)
	}

	return cooldown, nil
}

func cooldownUserKey(name string) string {
	return fmt.Sprintf("cooldown_%s", name)
}
