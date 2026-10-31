package requests

import (
	"github.com/thomas666-beast/reqval"
)

type ChatDM struct {
	reqval.Base

	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

func (r *ChatDM) Rules() map[string][]string {
	// Either user_id (uuid) OR username (3-32 chars) must be provided.
	// If both are given, user_id wins.
	return map[string][]string{}
}
