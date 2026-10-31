package profile

import (
	"context"
	"fmt"
)

type GeekStats struct {
	CompletedOrders int64   `json:"completed_orders"`
	ActiveProducts  int64   `json:"active_products"`
	AvgRating       float64 `json:"avg_rating"`
	ReviewCount     int64   `json:"review_count"`
}

func (s *Service) GetStats(ctx context.Context, userID string) (*GeekStats, error) {
	row, err := s.q.GetGeekStats(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get geek stats: %w", err)
	}
	return &GeekStats{
		CompletedOrders: row.CompletedOrders,
		ActiveProducts:  row.ActiveProducts,
		AvgRating:       row.AvgRating,
		ReviewCount:     row.ReviewCount,
	}, nil
}
