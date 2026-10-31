package product_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thomas666-beast/sindri/internal/db/dbgen"
	"github.com/thomas666-beast/sindri/internal/product"
	"github.com/thomas666-beast/sindri/internal/user"
)

func setup(t *testing.T) (*product.Service, *pgxpool.Pool, string) {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	us := user.NewService(dbgen.New(pool))
	suffix := time.Now().UnixNano()
	u, err := us.Register(context.Background(), user.RegisterInput{
		Email:    fmt.Sprintf("ptest_%d@example.com", suffix),
		Username: fmt.Sprintf("ptest_%d", suffix%1_000_000),
		Password: "hunter2hunter2",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM products WHERE seller_id=$1", u.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id=$1", u.ID)
	})
	return product.NewService(dbgen.New(pool)), pool, u.ID
}

func TestCreate_Success(t *testing.T) {
	svc, _, sellerID := setup(t)
	p, err := svc.Create(context.Background(), product.CreateInput{
		SellerID: sellerID, Title: "Vintage keyboard", Description: "IBM Model M",
		Price: 12500, CurrencyCode: "USD",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.Title != "Vintage keyboard" || p.Price != 12500 || p.CurrencyCode != "USD" {
		t.Errorf("unexpected product: %+v", p)
	}
	if p.SellerID != sellerID {
		t.Errorf("seller_id = %q, want %q", p.SellerID, sellerID)
	}
}

func TestCreate_RejectsBadInput(t *testing.T) {
	svc, _, sellerID := setup(t)
	cases := []product.CreateInput{
		{SellerID: sellerID, Title: "", Price: 100, CurrencyCode: "USD"},
		{SellerID: sellerID, Title: "ok", Price: 0, CurrencyCode: "USD"},
		{SellerID: sellerID, Title: "ok", Price: -1, CurrencyCode: "USD"},
		{SellerID: sellerID, Title: "ok", Price: 100, CurrencyCode: "US"},
	}
	for i, c := range cases {
		_, err := svc.Create(context.Background(), c)
		if !errors.Is(err, product.ErrInvalidInput) {
			t.Errorf("case %d: err = %v, want ErrInvalidInput", i, err)
		}
	}
}

func TestGet_NotFound(t *testing.T) {
	svc, _, _ := setup(t)
	_, err := svc.Get(context.Background(), "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, product.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestList_OrderedAndPaginated(t *testing.T) {
	svc, _, sellerID := setup(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if _, err := svc.Create(ctx, product.CreateInput{
			SellerID: sellerID, Title: fmt.Sprintf("item %d", i),
			Price: int64(100 + i), CurrencyCode: "USD",
		}); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}
	list, err := svc.List(ctx, "", 100, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) < 3 {
		t.Fatalf("expected >=3 products, got %d", len(list))
	}
}

func TestDeactivate_OnlyOwner(t *testing.T) {
	svc, pool, sellerID := setup(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, product.CreateInput{
		SellerID: sellerID, Title: "delete me", Price: 500, CurrencyCode: "USD",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := svc.Deactivate(ctx, p.ID, "00000000-0000-0000-0000-000000000000"); !errors.Is(err, product.ErrNotFound) {
		t.Fatalf("non-owner err = %v, want ErrNotFound", err)
	}
	var active bool
	_ = pool.QueryRow(ctx, `SELECT is_active FROM products WHERE id=$1`, p.ID).Scan(&active)
	if !active {
		t.Fatal("product should still be active after non-owner attempt")
	}

	if err := svc.Deactivate(ctx, p.ID, sellerID); err != nil {
		t.Fatalf("owner deactivate: %v", err)
	}
	_ = pool.QueryRow(ctx, `SELECT is_active FROM products WHERE id=$1`, p.ID).Scan(&active)
	if active {
		t.Fatal("product should be inactive after owner deactivate")
	}
}

// ---- Category integration tests (Task B3) ----

func TestCreate_WithCategory(t *testing.T) {
	svc, pool, sellerID := setup(t)
	ctx := context.Background()

	catID := createCategory(t, pool, "test-pc")

	p, err := svc.Create(ctx, product.CreateInput{
		SellerID: sellerID, Title: "Categorized product", Price: 1000,
		CurrencyCode: "USD", CategoryID: catID,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.CategoryID != catID {
		t.Errorf("category_id = %q, want %q", p.CategoryID, catID)
	}
}

func TestCreate_WithNonExistentCategory(t *testing.T) {
	svc, _, sellerID := setup(t)
	_, err := svc.Create(context.Background(), product.CreateInput{
		SellerID: sellerID, Title: "P", Price: 1000,
		CurrencyCode: "USD", CategoryID: "00000000-0000-0000-0000-000000000000",
	})
	if !errors.Is(err, product.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestList_FilterByCategory(t *testing.T) {
	svc, pool, sellerID := setup(t)
	ctx := context.Background()

	catA := createCategory(t, pool, "test-la")
	catB := createCategory(t, pool, "test-lb")

	if _, err := svc.Create(ctx, product.CreateInput{
		SellerID: sellerID, Title: "in A", Price: 100, CurrencyCode: "USD", CategoryID: catA,
	}); err != nil {
		t.Fatalf("create A: %v", err)
	}
	if _, err := svc.Create(ctx, product.CreateInput{
		SellerID: sellerID, Title: "in B", Price: 200, CurrencyCode: "USD", CategoryID: catB,
	}); err != nil {
		t.Fatalf("create B: %v", err)
	}
	if _, err := svc.Create(ctx, product.CreateInput{
		SellerID: sellerID, Title: "no cat", Price: 300, CurrencyCode: "USD",
	}); err != nil {
		t.Fatalf("create uncategorized: %v", err)
	}

	onlyA, err := svc.List(ctx, catA, 100, 0)
	if err != nil {
		t.Fatalf("list A: %v", err)
	}
	for _, p := range onlyA {
		if p.CategoryID != catA {
			t.Errorf("product %q in filter A has category %q", p.Title, p.CategoryID)
		}
	}
	if len(onlyA) < 1 {
		t.Error("expected at least 1 product in category A")
	}

	all, err := svc.List(ctx, "", 100, 0)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) < 3 {
		t.Errorf("unfiltered list should have >=3 test products, got %d", len(all))
	}
}

func TestSetCategory(t *testing.T) {
	svc, pool, sellerID := setup(t)
	ctx := context.Background()

	cat := createCategory(t, pool, "test-sc")
	p, err := svc.Create(ctx, product.CreateInput{
		SellerID: sellerID, Title: "P", Price: 100, CurrencyCode: "USD",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.CategoryID != "" {
		t.Fatalf("expected empty category, got %q", p.CategoryID)
	}

	if err := svc.SetCategory(ctx, p.ID, sellerID, cat); err != nil {
		t.Fatalf("set category: %v", err)
	}
	updated, err := svc.Get(ctx, p.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if updated.CategoryID != cat {
		t.Errorf("category_id = %q, want %q", updated.CategoryID, cat)
	}

	if err := svc.SetCategory(ctx, p.ID, sellerID, ""); err != nil {
		t.Fatalf("remove category: %v", err)
	}
	updated, err = svc.Get(ctx, p.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if updated.CategoryID != "" {
		t.Errorf("category_id = %q, want empty", updated.CategoryID)
	}
}

func TestSetCategory_NotOwner(t *testing.T) {
	svc, pool, sellerID := setup(t)
	ctx := context.Background()

	cat := createCategory(t, pool, "test-scn")
	p, err := svc.Create(ctx, product.CreateInput{
		SellerID: sellerID, Title: "P", Price: 100, CurrencyCode: "USD",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	err = svc.SetCategory(ctx, p.ID, "00000000-0000-0000-0000-000000000000", cat)
	if !errors.Is(err, product.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// ---- helpers ----

func createCategory(t *testing.T, pool *pgxpool.Pool, slugPrefix string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `
		INSERT INTO categories (slug, name) VALUES ($1, $1) RETURNING id::text
	`, fmt.Sprintf("%s-%d", slugPrefix, time.Now().UnixNano()%1_000_000_000)).Scan(&id)
	if err != nil {
		t.Fatalf("create category: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM products WHERE category_id=$1", id)
		_, _ = pool.Exec(context.Background(), "DELETE FROM categories WHERE id=$1", id)
	})
	return id
}

func TestUpdate_Success(t *testing.T) {
	svc, _, sellerID := setup(t)
	ctx := context.Background()

	p, err := svc.Create(ctx, product.CreateInput{
		SellerID: sellerID, Title: "Original", Description: "before",
		Price: 1000, CurrencyCode: "USD",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	upd, err := svc.Update(ctx, product.UpdateInput{
		ID: p.ID, SellerID: sellerID,
		Title: "Updated Title", Description: "after",
		Price: 2500,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if upd.Title != "Updated Title" {
		t.Errorf("title = %q", upd.Title)
	}
	if upd.Description != "after" {
		t.Errorf("description = %q", upd.Description)
	}
	if upd.Price != 2500 {
		t.Errorf("price = %d", upd.Price)
	}
}

func TestUpdate_NotOwner(t *testing.T) {
	svc, _, sellerID := setup(t)
	ctx := context.Background()
	p, _ := svc.Create(ctx, product.CreateInput{
		SellerID: sellerID, Title: "P", Price: 1000, CurrencyCode: "USD",
	})

	_, err := svc.Update(ctx, product.UpdateInput{
		ID: p.ID, SellerID: "00000000-0000-0000-0000-000000000000",
		Title: "Hijack", Price: 100,
	})
	if !errors.Is(err, product.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdate_RejectsBadInput(t *testing.T) {
	svc, _, sellerID := setup(t)
	ctx := context.Background()
	p, _ := svc.Create(ctx, product.CreateInput{
		SellerID: sellerID, Title: "P", Price: 1000, CurrencyCode: "USD",
	})

	for _, in := range []product.UpdateInput{
		{ID: p.ID, SellerID: sellerID, Title: "", Price: 100},
		{ID: p.ID, SellerID: sellerID, Title: "P", Price: 0},
		{ID: p.ID, SellerID: sellerID, Title: "P", Price: -5},
	} {
		_, err := svc.Update(ctx, in)
		if !errors.Is(err, product.ErrInvalidInput) {
			t.Errorf("input %+v: err = %v, want ErrInvalidInput", in, err)
		}
	}
}

func TestUpdate_DeactivatedProduct(t *testing.T) {
	svc, _, sellerID := setup(t)
	ctx := context.Background()
	p, _ := svc.Create(ctx, product.CreateInput{
		SellerID: sellerID, Title: "P", Price: 1000, CurrencyCode: "USD",
	})
	if err := svc.Deactivate(ctx, p.ID, sellerID); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	_, err := svc.Update(ctx, product.UpdateInput{
		ID: p.ID, SellerID: sellerID, Title: "Changed", Price: 500,
	})
	if !errors.Is(err, product.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound (deactivated)", err)
	}
}
