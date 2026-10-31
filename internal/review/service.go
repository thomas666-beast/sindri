package review

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/thomas666-beast/sindri/internal/db/dbgen"
)

var (
	ErrNotFound         = errors.New("review not found")
	ErrOrderNotFound    = errors.New("order not found")
	ErrNotBuyer         = errors.New("only the buyer can review this order")
	ErrOrderNotCompleted = errors.New("order is not completed; cannot review")
	ErrAlreadyReviewed  = errors.New("order already reviewed")
	ErrInvalidRating    = errors.New("rating must be between 1 and 5")
	ErrInvalidComment   = errors.New("comment must be at most 2000 characters")
)

type Review struct {
	ID            string `json:"id"`
	OrderID       string `json:"order_id"`
	BuyerID       string `json:"buyer_id"`
	BuyerUsername string `json:"buyer_username,omitempty"`
	SellerID      string `json:"seller_id"`
	ProductID     string `json:"product_id,omitempty"`
	Rating        int16  `json:"rating"`
	Comment       string `json:"comment"`
	CreatedAtUnix int64  `json:"created_at_unix"`
}

type Summary struct {
	AvgRating   float64 `json:"avg_rating"`
	ReviewCount int64   `json:"review_count"`
}

type Page struct {
	Reviews []Review `json:"reviews"`
	Total   int64    `json:"total"`
	Limit   int32    `json:"limit"`
	Offset  int32    `json:"offset"`
}

type Service struct {
	q dbgen.Querier
}

func NewService(q dbgen.Querier) *Service { return &Service{q: q} }

type CreateInput struct {
	OrderID string
	BuyerID string
	Rating  int16
	Comment string
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Review, error) {
	if in.Rating < 1 || in.Rating > 5 {
		return nil, ErrInvalidRating
	}
	if len(in.Comment) > 2000 {
		return nil, ErrInvalidComment
	}

	// Look up the order to verify buyer + status + get seller/product.
	o, err := s.q.GetOrder(ctx, in.OrderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOrderNotFound
		}
		return nil, fmt.Errorf("get order: %w", err)
	}
	if o.BuyerID != in.BuyerID {
		return nil, ErrNotBuyer
	}
	if o.Status != "completed" {
		return nil, ErrOrderNotCompleted
	}

	// Reject if already reviewed (unique constraint would catch it too, but
	// this gives a cleaner error).
	if _, err := s.q.GetReviewByOrder(ctx, in.OrderID); err == nil {
		return nil, ErrAlreadyReviewed
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("check existing review: %w", err)
	}

	row, err := s.q.CreateReview(ctx, dbgen.CreateReviewParams{
		OrderID:   in.OrderID,
		BuyerID:   in.BuyerID,
		SellerID:  o.SellerID,
		ProductID: o.ProductID,
		Rating:    in.Rating,
		Comment:   in.Comment,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrAlreadyReviewed
		}
		return nil, fmt.Errorf("create review: %w", err)
	}

	return toDTO(row.ID, row.OrderID, row.BuyerID, "", row.SellerID,
		pgUUIDString(row.ProductID), row.Rating, row.Comment, row.CreatedAt.Unix()), nil
}

func (s *Service) ListBySeller(ctx context.Context, sellerID string, limit, offset int32) (*Page, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := s.q.ListReviewsBySeller(ctx, dbgen.ListReviewsBySellerParams{
		SellerID: sellerID, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list by seller: %w", err)
	}
	total, err := s.q.SellerRatingSummary(ctx, sellerID)
	if err != nil {
		return nil, fmt.Errorf("summary: %w", err)
	}

	out := make([]Review, 0, len(rows))
	for _, r := range rows {
		out = append(out, *toDTO(r.ID, r.OrderID, r.BuyerID, r.BuyerUsername, r.SellerID,
			pgUUIDString(r.ProductID), r.Rating, r.Comment, r.CreatedAt.Unix()))
	}
	return &Page{Reviews: out, Total: total.ReviewCount, Limit: limit, Offset: offset}, nil
}

func (s *Service) ListByProduct(ctx context.Context, productID string, limit, offset int32) (*Page, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := s.q.ListReviewsByProduct(ctx, dbgen.ListReviewsByProductParams{
		ProductID: pgUUID(productID), Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list by product: %w", err)
	}
	total, err := s.q.ProductRatingSummary(ctx, pgUUID(productID))
	if err != nil {
		return nil, fmt.Errorf("summary: %w", err)
	}

	out := make([]Review, 0, len(rows))
	for _, r := range rows {
		out = append(out, *toDTO(r.ID, r.OrderID, r.BuyerID, r.BuyerUsername, r.SellerID,
			pgUUIDString(r.ProductID), r.Rating, r.Comment, r.CreatedAt.Unix()))
	}
	return &Page{Reviews: out, Total: total.ReviewCount, Limit: limit, Offset: offset}, nil
}

func (s *Service) SellerSummary(ctx context.Context, sellerID string) (*Summary, error) {
	row, err := s.q.SellerRatingSummary(ctx, sellerID)
	if err != nil {
		return nil, fmt.Errorf("seller summary: %w", err)
	}
	return summaryFrom(row.RatingSum, row.ReviewCount), nil
}

func (s *Service) ProductSummary(ctx context.Context, productID string) (*Summary, error) {
	row, err := s.q.ProductRatingSummary(ctx, pgUUID(productID))
	if err != nil {
		return nil, fmt.Errorf("product summary: %w", err)
	}
	return summaryFrom(row.RatingSum, row.ReviewCount), nil
}

func summaryFrom(sum, count int64) *Summary {
	avg := 0.0
	if count > 0 {
		avg = float64(sum) / float64(count)
	}
	return &Summary{AvgRating: avg, ReviewCount: count}
}

// ---- helpers ----

func toDTO(id, orderID, buyerID, buyerUsername, sellerID, productID string, rating int16, comment string, createdAt int64) *Review {
	return &Review{
		ID: id, OrderID: orderID, BuyerID: buyerID, BuyerUsername: buyerUsername,
		SellerID: sellerID, ProductID: productID, Rating: rating,
		Comment: comment, CreatedAtUnix: createdAt,
	}
}

func pgUUID(s string) pgtype.UUID {
	var u pgtype.UUID
	_ = u.Scan(s)
	return u
}

func pgUUIDString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	s, _ := u.Value()
	if str, ok := s.(string); ok {
		return str
	}
	return ""
}

func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.SQLState() == "23505"
}
