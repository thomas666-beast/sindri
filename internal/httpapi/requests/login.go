package requests

import (
	"context"

	"github.com/thomas666-beast/reqval"
)

type Login struct {
	reqval.Base

	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r *Login) Rules() map[string][]string {
	return map[string][]string{
		"Email":    {"required", "email"},
		"Password": {"required"},
	}
}

func (r *Login) Authorize(ctx context.Context) error {
	return nil
}
