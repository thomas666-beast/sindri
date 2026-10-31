package category

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/thomas666-beast/sindri/internal/db/dbgen"
)

var (
	ErrNotFound        = errors.New("category not found")
	ErrInvalidInput    = errors.New("invalid category input")
	ErrSlugTaken       = errors.New("slug already used")
	ErrHasChildren     = errors.New("category has subcategories; delete them first")
	ErrParentHasParent = errors.New("parent category is itself a subcategory; only one level allowed")
	ErrParentIsSelf    = errors.New("category cannot be its own parent")
)

type Category struct {
	ID            string `json:"id"`
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	SortOrder     int32  `json:"sort_order"`
	IsActive      bool   `json:"is_active"`
	ParentID      string `json:"parent_id,omitempty"`
	CreatedAtUnix int64  `json:"created_at_unix"`
	UpdatedAtUnix int64  `json:"updated_at_unix"`
}

type Service struct {
	q dbgen.Querier
}

func NewService(q dbgen.Querier) *Service { return &Service{q: q} }

type CreateInput struct {
	Slug        string
	Name        string
	Description string
	SortOrder   int32
	ParentID    string
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Category, error) {
	in.Slug = strings.TrimSpace(strings.ToLower(in.Slug))
	in.Name = strings.TrimSpace(in.Name)
	if !validSlug(in.Slug) {
		return nil, fmt.Errorf("%w: slug must be 2-64 lowercase letters, digits, hyphens", ErrInvalidInput)
	}
	if len(in.Name) < 1 || len(in.Name) > 100 {
		return nil, fmt.Errorf("%w: name must be 1-100 chars", ErrInvalidInput)
	}
	if len(in.Description) > 2000 {
		return nil, fmt.Errorf("%w: description too long", ErrInvalidInput)
	}

	var parentUUID pgtype.UUID
	if in.ParentID != "" {
		parent, err := s.q.GetCategoryByID(ctx, in.ParentID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf("%w: parent not found", ErrInvalidInput)
			}
			return nil, fmt.Errorf("get parent: %w", err)
		}
		if parent.ParentID.Valid {
			return nil, ErrParentHasParent
		}
		parentUUID = pgUUID(in.ParentID)
	}

	row, err := s.q.CreateCategory(ctx, dbgen.CreateCategoryParams{
		Slug:        in.Slug,
		Name:        in.Name,
		Description: in.Description,
		SortOrder:   in.SortOrder,
		ParentID:    parentUUID,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrSlugTaken
		}
		return nil, fmt.Errorf("create category: %w", err)
	}
	return toCategory(
		row.ID, row.Slug, row.Name, row.Description,
		row.SortOrder, row.IsActive, row.ParentID,
		row.CreatedAt, row.UpdatedAt,
	), nil
}

func (s *Service) Get(ctx context.Context, id string) (*Category, error) {
	row, err := s.q.GetCategoryByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get category: %w", err)
	}
	return toCategory(
		row.ID, row.Slug, row.Name, row.Description,
		row.SortOrder, row.IsActive, row.ParentID,
		row.CreatedAt, row.UpdatedAt,
	), nil
}

func (s *Service) GetBySlug(ctx context.Context, slug string) (*Category, error) {
	row, err := s.q.GetCategoryBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get category: %w", err)
	}
	return toCategory(
		row.ID, row.Slug, row.Name, row.Description,
		row.SortOrder, row.IsActive, row.ParentID,
		row.CreatedAt, row.UpdatedAt,
	), nil
}

func (s *Service) List(ctx context.Context, onlyActive bool, parentFilter string, limit, offset int32) ([]Category, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	if parentFilter != "" && parentFilter != "top" && parentFilter != "children" {
		return nil, fmt.Errorf("%w: parent_filter must be top, children, or empty", ErrInvalidInput)
	}
	rows, err := s.q.ListCategories(ctx, dbgen.ListCategoriesParams{
		OnlyActive:   onlyActive,
		ParentFilter: parentFilter,
		Lim:          limit,
		Off:          offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	out := make([]Category, 0, len(rows))
	for _, r := range rows {
		out = append(out, *toCategory(
			r.ID, r.Slug, r.Name, r.Description,
			r.SortOrder, r.IsActive, r.ParentID,
			r.CreatedAt, r.UpdatedAt,
		))
	}
	return out, nil
}

func (s *Service) ListChildren(ctx context.Context, parentID string, onlyActive bool, limit, offset int32) ([]Category, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.q.ListChildren(ctx, dbgen.ListChildrenParams{
		ParentID:   pgUUID(parentID),
		OnlyActive: onlyActive,
		Lim:        limit,
		Off:        offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list children: %w", err)
	}
	out := make([]Category, 0, len(rows))
	for _, r := range rows {
		out = append(out, *toCategory(
			r.ID, r.Slug, r.Name, r.Description,
			r.SortOrder, r.IsActive, r.ParentID,
			r.CreatedAt, r.UpdatedAt,
		))
	}
	return out, nil
}

type UpdateInput struct {
	ID          string
	Slug        string
	Name        string
	Description string
	SortOrder   int32
	IsActive    bool
}

func (s *Service) Update(ctx context.Context, in UpdateInput) (*Category, error) {
	in.Slug = strings.TrimSpace(strings.ToLower(in.Slug))
	in.Name = strings.TrimSpace(in.Name)
	if !validSlug(in.Slug) {
		return nil, fmt.Errorf("%w: invalid slug", ErrInvalidInput)
	}
	if len(in.Name) < 1 || len(in.Name) > 100 {
		return nil, fmt.Errorf("%w: name must be 1-100 chars", ErrInvalidInput)
	}

	row, err := s.q.UpdateCategory(ctx, dbgen.UpdateCategoryParams{
		ID:          in.ID,
		Slug:        in.Slug,
		Name:        in.Name,
		Description: in.Description,
		SortOrder:   in.SortOrder,
		IsActive:    in.IsActive,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if isUniqueViolation(err) {
			return nil, ErrSlugTaken
		}
		return nil, fmt.Errorf("update category: %w", err)
	}
	return toCategory(
		row.ID, row.Slug, row.Name, row.Description,
		row.SortOrder, row.IsActive, row.ParentID,
		row.CreatedAt, row.UpdatedAt,
	), nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	count, err := s.q.CountChildren(ctx, pgUUID(id))
	if err != nil {
		return fmt.Errorf("count children: %w", err)
	}
	if count > 0 {
		return ErrHasChildren
	}
	n, err := s.q.DeleteCategory(ctx, id)
	if err != nil {
		return fmt.Errorf("delete category: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- helpers ----

func validSlug(s string) bool {
	if len(s) < 2 || len(s) > 64 {
		return false
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return false
		}
	}
	return true
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

func toCategory(
	id, slug, name, desc string,
	sortOrder int32, isActive bool,
	parentID pgtype.UUID,
	createdAt, updatedAt time.Time,
) *Category {
	return &Category{
		ID:            id,
		Slug:          slug,
		Name:          name,
		Description:   desc,
		SortOrder:     sortOrder,
		IsActive:      isActive,
		ParentID:      pgUUIDString(parentID),
		CreatedAtUnix: createdAt.Unix(),
		UpdatedAtUnix: updatedAt.Unix(),
	}
}
