package product

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/thomas666-beast/sindri/internal/db/dbgen"
)

type SearchResult struct {
	ID             string `json:"id"`
	SellerID       string `json:"seller_id"`
	SellerUsername string `json:"seller_username"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	Price          int64  `json:"price"`
	CurrencyCode   string `json:"currency_code"`
	CategoryID     string `json:"category_id,omitempty"`
	CategorySlug   string `json:"category_slug,omitempty"`
	CategoryName   string `json:"category_name,omitempty"`
	CoverImage     string `json:"cover_image,omitempty"`
	CreatedAtUnix  int64  `json:"created_at_unix"`
}

type SearchPage struct {
	Results []SearchResult `json:"results"`
	Total   int64          `json:"total"`
	Limit   int32          `json:"limit"`
	Offset  int32          `json:"offset"`
}

func (s *Service) Search(ctx context.Context, q, categoryID, sellerID, idFilter string, limit, offset int32) (*SearchPage, error) {
	q = strings.TrimSpace(q)
	idFilter = strings.TrimSpace(idFilter)
	
	if len(q) > 200 {
		q = q[:200]
	}
	if len(idFilter) > 36 {
		idFilter = idFilter[:36]
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}

	var catUUID, sellerUUID pgtype.UUID
	if categoryID != "" {
		catUUID = pgUUID(categoryID)
	}
	if sellerID != "" {
		sellerUUID = pgUUID(sellerID)
	}

	rows, err := s.q.SearchProducts(ctx, dbgen.SearchProductsParams{
		Q:          q,
		CategoryID: catUUID,
		SellerID:   sellerUUID,
		IDFilter:   idFilter,
		Lim:        limit,
		Off:        offset,
	})
	if err != nil {
		return nil, fmt.Errorf("search products: %w", err)
	}

	total, err := s.q.CountSearchProducts(ctx, dbgen.CountSearchProductsParams{
		Q:          q,
		CategoryID: catUUID,
		SellerID:   sellerUUID,
		IDFilter:   idFilter,
	})
	if err != nil {
		return nil, fmt.Errorf("count search: %w", err)
	}

	results := make([]SearchResult, 0, len(rows))
	for _, r := range rows {
		results = append(results, SearchResult{
			ID:             r.ID,
			SellerID:       r.SellerID,
			SellerUsername: r.SellerUsername,
			Title:          r.Title,
			Description:    r.Description,
			Price:          r.Price,
			CurrencyCode:   r.CurrencyCode,
			CategoryID:     pgUUIDString(r.CategoryID),
			CategorySlug:   r.CategorySlug,
			CategoryName:   r.CategoryName,
			CoverImage:     r.CoverImage, // It's just a string now!
			CreatedAtUnix:  r.CreatedAt.Unix(),
		})
	}

	return &SearchPage{
		Results: results,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
	}, nil
}
