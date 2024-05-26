package user

import (
	"encoding/json"
	"fmt"
)

type User interface {
	String() string
	Unmarshal([]byte) error
}

type user struct {
	Name string `json:"name"`
}

func New() *user {
	return &user{}
}

func (u *user) String() string {
	return fmt.Sprintf("User{name: %s}", u.Name)
}

func (u *user) Unmarshal(data []byte) error {
	if err := json.Unmarshal(data, u); err != nil {
		return fmt.Errorf("error unmarshalling: %w", err)
	}

	return nil
}
