package pixel

import (
	"encoding/json"
	"fmt"
)

type Pixel interface {
	String() string
	GetX() int
	GetY() int
	GetColor() string
	Marshal() ([]byte, error)
	Unmarshal([]byte) error
}

type pixel struct {
	X     int    `json:"x"`
	Y     int    `json:"y"`
	Color string `json:"color,omitempty"`
}

func New() *pixel {
	return &pixel{}
}

func (p *pixel) String() string {
	return fmt.Sprintf("Pixel{x: %d, y: %d, color: %s}", p.X, p.Y, p.Color)
}

func (p *pixel) Unmarshal(data []byte) error {
	if err := json.Unmarshal(data, p); err != nil {
		return fmt.Errorf("error unmarshaling pixel: %w", err)
	}

	return nil
}

func (p *pixel) Marshal() ([]byte, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("error marshaling pixel: %w", err)
	}

	return data, nil
}

func (p *pixel) GetX() int {
	return p.X
}

func (p *pixel) GetY() int {
	return p.Y
}

func (p *pixel) GetColor() string {
	return p.Color
}
