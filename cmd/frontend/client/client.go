package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	log.Printf("received user info: %s", u)

	// go into a loop of handling client messages
	for {
		// Read the message from the client
		if err := c.conn.ReadJSON(&req); err != nil {
			return fmt.Errorf("error reading request: %w", err)
		}
		log.Printf("received request from client: %+v", req)

		switch req.Type {
		case RequestTypePut:
			p := pixel.New()
			if err := p.Unmarshal([]byte(req.Data)); err != nil {
				log.Println("error unmarshalling pixel:", err)
				break
			}
			log.Printf("received put pixel request: %s", p)

			data := PixelMetadata{
				Pixel: p,
				User:  u,
			}
			if err := c.savePixel(ctx, data); err != nil {
				log.Println("error saving pixel:", err)
				break
			}

			// broadcast event to all clients
			if err := c.broadcast(ctx, data); err != nil {
				log.Println("error broadcasting pixel:", err)
				break
			}

		case RequestTypePixelInfo:
			p := pixel.New()
			if err := p.Unmarshal([]byte(req.Data)); err != nil {
				log.Println("error unmarshalling pixel:", err)
				break
			}
			log.Printf("received pixel info request: %s", p)

			// fetch the pixel data, encode it into json and send it back
			data, err := c.getPixel(ctx, p)
			if err != nil && err != ErrPixelNotFound {
				log.Println("Error getting pixel info:", err)
				break
			}
			if err == ErrPixelNotFound {
				break
			}

			jsonData, err := json.Marshal(data)
			if err != nil {
				log.Println("Error marshalling pixel info:", err)
				break
			}

			// send back the reply
			if err := c.Send(Event{
				Type: EventTypePixelInfo,
				Data: string(jsonData),
			}); err != nil {
				log.Println("error sending pixel info:", err)
				break
			}

		case RequestTypeCanvas:
			log.Printf("received canvas request")

			// get the canvas
			canvas, err := c.getCanvas(ctx)
			if err != nil {
				log.Println("Error getting canvas:", err)
				break
			}

			jsonData, err := json.Marshal(canvas)
			if err != nil {
				log.Println("Error marshalling canvas:", err)
				break
			}

			// send back the reply
			if err := c.Send(Event{
				Type: EventTypeCanvas,
				Data: string(jsonData),
			}); err != nil {
				log.Println("error sending canvas:", err)
				break
			}
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

func (c *client) savePixel(ctx context.Context, data PixelMetadata) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error marshalling pixel data: %w", err)
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
	query := "{}"
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	resp, err := c.dapr.QueryStateAlpha1(ctx, c.cfg.StateStore.Name, query, nil)
	if err != nil {
		return nil, fmt.Errorf("error getting pixel data: %w", err)
	}

	var pixels []pixel.Pixel
	for _, item := range resp.Results {
		var m PixelMetadata
		if err := json.Unmarshal(item.Value, &m); err != nil {
			continue
		}

		pixels = append(pixels, m.Pixel)
	}

	return pixels, nil
}
