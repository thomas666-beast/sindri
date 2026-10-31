package requests

import (

	"github.com/thomas666-beast/reqval"
)

type ChatMessage struct {
	reqval.Base

	Body string `json:"body"`
}

func (r *ChatMessage) Rules() map[string][]string {
	return map[string][]string{
		"Body": {"required", "min:1", "max:10000"},
	}
}
