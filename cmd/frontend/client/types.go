package client

import (
	"encoding/json"
	"fmt"

	"github.com/lrascao/place/pkg/pixel"
	"github.com/lrascao/place/pkg/user"
)

type RequestType string

const (
	RequestTypeUserInfo  RequestType = "userinfo"
	RequestTypePut       RequestType = "put"
	RequestTypePixelInfo RequestType = "pixelinfo"
	RequestTypeCanvas    RequestType = "canvas"
)

type EventType string

const (
	EventTypePut       EventType = "put"
	EventTypePixelInfo EventType = "pixelinfo"
	EventTypeCanvas    EventType = "canvas"
)

type Request struct {
	Type RequestType `json:"type"`
	Data string      `json:"data"`
}

type Event struct {
	Type EventType `json:"type"`
	Data string    `json:"data"`
}

type PixelMetadata struct {
	pixel.Pixel `json:"pixel"`
	user.User   `json:"user"`
}

func (m *PixelMetadata) UnmarshalJSON(data []byte) error {
	raw := make(map[string]json.RawMessage)
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("error unmarshalling pixel metadata: %w", err)
	}

	p := pixel.New()
	if err := p.Unmarshal(raw["pixel"]); err != nil {
		return fmt.Errorf("error unmarshalling pixel metadata: %w", err)
	}

	u := user.New()
	if err := u.Unmarshal(raw["user"]); err != nil {
		return fmt.Errorf("error unmarshalling user metadata: %w", err)
	}

	m.Pixel = p
	m.User = u

	return nil
}
