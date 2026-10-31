package settings

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/thomas666-beast/sindri/internal/db/dbgen"
)

type Service struct {
	q dbgen.Querier
}

func NewService(q dbgen.Querier) *Service {
	return &Service{q: q}
}

func (s *Service) Bool(ctx context.Context, key string, def bool) (bool, error) {
	raw, err := s.q.GetSetting(ctx, key)
	if err != nil {
		return def, nil // missing setting → use default
	}
	var v bool
	if err := json.Unmarshal(raw, &v); err != nil {
		return def, fmt.Errorf("setting %q not a bool: %w", key, err)
	}
	return v, nil
}

func (s *Service) Int(ctx context.Context, key string, def int) (int, error) {
	raw, err := s.q.GetSetting(ctx, key)
	if err != nil {
		return def, nil
	}
	var v int
	if err := json.Unmarshal(raw, &v); err != nil {
		return def, fmt.Errorf("setting %q not an int: %w", key, err)
	}
	return v, nil
}
