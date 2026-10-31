package requests

import (
	"github.com/thomas666-beast/reqval"
)

type ChatGroup struct {
	reqval.Base

	Title     string   `json:"title"`
	Members   []string `json:"members"`   // UUIDs
	Usernames []string `json:"usernames"` // usernames
}

func (r *ChatGroup) Rules() map[string][]string {
	return map[string][]string{
		"Title": {"required", "min:1", "max:200"},
	}
}
