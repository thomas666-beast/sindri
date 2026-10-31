package product

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/thomas666-beast/sindri/internal/db/dbgen"
)

var (
	ErrInvalidInput = errors.New("invalid product input")
	ErrNotFound     = errors.New("product not found")
	ErrNotOwner     = errors.New("not the product owner")
)

type Product struct {
	ID             string `json:"id"`
	SellerID       string `json:"seller_id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	Price          int64  `json:"price"`
	CurrencyCode   string `json:"currency_code"`
	CategoryID     string `json:"category_id,omitempty"`
	CreatedAtUnix  int64  `json:"created_at_unix"`
	SellerUsername string `json:"seller_username,omitempty"`
	CoverImage     string `json:"cover_image,omitempty"`
}

type Service struct {
	q dbgen.Querier
}

func NewService(q dbgen.Querier) *Service { return &Service{q: q} }

type CreateInput struct {
	SellerID     string
	Title        string
	Description  string
	Price        int64
	CurrencyCode string
	CategoryID   string
}

type UpdateInput struct {
	ID          string
	SellerID    string
	Title       string
	Description string
	Price       int64
	CategoryID  string
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Product, error) {
	if in.Title == "" || len(in.Title) > 200 {
		return nil, ErrInvalidInput
	}
	if in.Price <= 0 {
		return nil, ErrInvalidInput
	}
	if len(in.CurrencyCode) != 3 {
		return nil, ErrInvalidInput
	}

	var categoryUUID pgtype.UUID
	if in.CategoryID != "" {
		// Verify the category exists.
		_, err := s.q.GetCategoryByID(ctx, in.CategoryID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf("%w: category not found", ErrInvalidInput)
			}
			return nil, fmt.Errorf("get category: %w", err)
		}
		categoryUUID = pgUUID(in.CategoryID)
	}

	row, err := s.q.CreateProduct(ctx, dbgen.CreateProductParams{
		SellerID:     in.SellerID,
		Title:        in.Title,
		Description:  in.Description,
		Price:        in.Price,
		CurrencyCode: in.CurrencyCode,
		CategoryID:   categoryUUID,
	})
	if err != nil {
		return nil, fmt.Errorf("create product: %w", err)
	}
	return toProduct(
		row.ID, row.SellerID, row.Title, row.Description,
		row.Price, row.CurrencyCode, row.CategoryID, row.CreatedAt,
	), nil
}

func (s *Service) Get(ctx context.Context, id string) (*Product, error) {
	row, err := s.q.GetProduct(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get product: %w", err)
	}
	p := toProduct(row.ID, row.SellerID, row.Title, row.Description,
		row.Price, row.CurrencyCode, row.CategoryID, row.CreatedAt)
	p.SellerUsername = row.SellerUsername
	return p, nil
}

func (s *Service) List(ctx context.Context, categoryID string, limit, offset int32) ([]Product, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	var catUUID pgtype.UUID
	if categoryID != "" {
		catUUID = pgUUID(categoryID)
	}

	rows, err := s.q.ListProducts(ctx, dbgen.ListProductsParams{
		CategoryID: catUUID,
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	out := make([]Product, 0, len(rows))
	for _, r := range rows {
		out = append(out, *toProduct(
			r.ID, r.SellerID, r.Title, r.Description,
			r.Price, r.CurrencyCode, r.CategoryID, r.CreatedAt,
		))
	}
	return out, nil
}

func (s *Service) ListBySeller(ctx context.Context, sellerID string, limit, offset int32) ([]Product, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.q.ListProductsBySeller(ctx, dbgen.ListProductsBySellerParams{
		SellerID: sellerID, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list by seller: %w", err)
	}
	out := make([]Product, 0, len(rows))
	for _, r := range rows {
		out = append(out, *toProduct(
			r.ID, r.SellerID, r.Title, r.Description,
			r.Price, r.CurrencyCode, r.CategoryID, r.CreatedAt,
		))
	}
	return out, nil
}

func (s *Service) Deactivate(ctx context.Context, id, requesterID string) error {
	n, err := s.q.DeactivateProduct(ctx, dbgen.DeactivateProductParams{
		ID: id, SellerID: requesterID,
	})
	if err != nil {
		return fmt.Errorf("deactivate: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) SetCategory(ctx context.Context, productID, sellerID, categoryID string) error {
	var catUUID pgtype.UUID
	if categoryID != "" {
		_, err := s.q.GetCategoryByID(ctx, categoryID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("%w: category not found", ErrInvalidInput)
			}
			return fmt.Errorf("get category: %w", err)
		}
		catUUID = pgUUID(categoryID)
	}

	n, err := s.q.SetProductCategory(ctx, dbgen.SetProductCategoryParams{
		ID:         productID,
		CategoryID: catUUID,
		SellerID:   sellerID,
	})
	if err != nil {
		return fmt.Errorf("set category: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) Update(ctx context.Context, in UpdateInput) (*Product, error) {
	if in.Title == "" || len(in.Title) > 200 {
		return nil, ErrInvalidInput
	}
	if in.Price <= 0 {
		return nil, ErrInvalidInput
	}

	var categoryUUID pgtype.UUID
	if in.CategoryID != "" {
		_, err := s.q.GetCategoryByID(ctx, in.CategoryID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf("%w: category not found", ErrInvalidInput)
			}
			return nil, fmt.Errorf("get category: %w", err)
		}
		categoryUUID = pgUUID(in.CategoryID)
	}

	row, err := s.q.UpdateProduct(ctx, dbgen.UpdateProductParams{
		ID:          in.ID,
		SellerID:    in.SellerID,
		Title:       in.Title,
		Description: in.Description,
		Price:       in.Price,
		CategoryID:  categoryUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update product: %w", err)
	}
	return toProduct(
		row.ID, row.SellerID, row.Title, row.Description,
		row.Price, row.CurrencyCode, row.CategoryID, row.CreatedAt,
	), nil
}

// AddImage saves the uploaded image URL to the database for a product.
func (s *Service) AddImage(ctx context.Context, productID, sellerID, url string, order int32) error {
	// Verify seller owns product
	p, err := s.Get(ctx, productID)
	if err != nil {
		return err
	}
	if p.SellerID != sellerID {
		return ErrNotOwner
	}

	if err := s.q.AddProductImage(ctx, dbgen.AddProductImageParams{
		ProductID: productID,
		Url:       url,
		SortOrder: order,
	}); err != nil {
		return fmt.Errorf("add image: %w", err)
	}
	return nil
}

// GetImages fetches all image URLs for a product.
func (s *Service) GetImages(ctx context.Context, productID string) ([]string, error) {
	rows, err := s.q.ListProductImages(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Url)
	}
	return out, nil
}

// ---- helpers ----

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

func toProduct(
	id, sellerID, title, desc string,
	price int64, currency string,
	categoryID pgtype.UUID,
	createdAt time.Time,
) *Product {
	return &Product{
		ID:            id,
		SellerID:      sellerID,
		Title:         title,
		Description:   desc,
		Price:         price,
		CurrencyCode:  currency,
		CategoryID:    pgUUIDString(categoryID),
		CreatedAtUnix: createdAt.Unix(),
	}
}
