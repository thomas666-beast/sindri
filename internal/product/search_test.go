package product_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thomas666-beast/sindri/internal/product"
)

func TestSearch_ByTitle(t *testing.T) {
	svc, _, sellerID := setup(t)
	ctx := context.Background()

	uniq := fmt.Sprintf("uniquetitle%d", time.Now().UnixNano()%1_000_000)
	if _, err := svc.Create(ctx, product.CreateInput{
		SellerID: sellerID, Title: "Awesome " + uniq,
		Price: 1000, CurrencyCode: "USD",
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	page, err := svc.Search(ctx, uniq, "", "", "", 30, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(page.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(page.Results))
	}
	if !strings.Contains(page.Results[0].Title, uniq) {
		t.Errorf("title = %q, missing %q", page.Results[0].Title, uniq)
	}
}

func TestSearch_ByDescription(t *testing.T) {
	svc, _, sellerID := setup(t)
	ctx := context.Background()

	uniq := fmt.Sprintf("uniquedesc%d", time.Now().UnixNano()%1_000_000)
	if _, err := svc.Create(ctx, product.CreateInput{
		SellerID:    sellerID,
		Title:       "Some product",
		Description: "This has " + uniq + " inside",
		Price:       500,
		CurrencyCode: "USD",
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	page, err := svc.Search(ctx, uniq, "", "", "", 30, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(page.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(page.Results))
	}
	if !strings.Contains(page.Results[0].Description, uniq) {
		t.Errorf("description = %q, missing %q", page.Results[0].Description, uniq)
	}
}

func TestSearch_CaseInsensitive(t *testing.T) {
	svc, _, sellerID := setup(t)
	ctx := context.Background()

	uniq := fmt.Sprintf("CASETEST%d", time.Now().UnixNano()%1_000_000)
	if _, err := svc.Create(ctx, product.CreateInput{
		SellerID: sellerID, Title: "The " + uniq + " item",
		Price: 100, CurrencyCode: "USD",
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	page, err := svc.Search(ctx, strings.ToLower(uniq), "", "", "", 30, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(page.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(page.Results))
	}
}

func TestSearch_ByCategory(t *testing.T) {
	svc, pool, sellerID := setup(t)
	ctx := context.Background()

	catID := createCategory(t, pool, "test-searchcat")

	if _, err := svc.Create(ctx, product.CreateInput{
		SellerID: sellerID, Title: "In category",
		Price: 100, CurrencyCode: "USD", CategoryID: catID,
	}); err != nil {
		t.Fatalf("create in cat: %v", err)
	}
	if _, err := svc.Create(ctx, product.CreateInput{
		SellerID: sellerID, Title: "Not in category",
		Price: 200, CurrencyCode: "USD",
	}); err != nil {
		t.Fatalf("create no cat: %v", err)
	}

	page, err := svc.Search(ctx, "", catID, "", "", 30, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(page.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(page.Results))
	}
	if page.Results[0].CategoryID != catID {
		t.Errorf("category_id = %q, want %q", page.Results[0].CategoryID, catID)
	}
}

func TestSearch_BySubcategory(t *testing.T) {
	svc, pool, sellerID := setup(t)
	ctx := context.Background()

	// Create parent and child categories
	parentID := createCategory(t, pool, "test-parent")
	childID := createSubcategory(t, pool, "test-child", parentID)

	if _, err := svc.Create(ctx, product.CreateInput{
		SellerID: sellerID, Title: "In subcategory",
		Price: 100, CurrencyCode: "USD", CategoryID: childID,
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Search by PARENT category should find the product in the child.
	page, err := svc.Search(ctx, "", parentID, "", "", 30, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(page.Results) != 1 {
		t.Fatalf("expected 1 result in parent search, got %d", len(page.Results))
	}
}

func TestSearch_BySeller(t *testing.T) {
	svc, pool, sellerID := setup(t)
	ctx := context.Background()

	// Second seller
	otherSellerID := createOtherSeller(t, pool)

	if _, err := svc.Create(ctx, product.CreateInput{
		SellerID: sellerID, Title: "From seller A",
		Price: 100, CurrencyCode: "USD",
	}); err != nil {
		t.Fatalf("create A: %v", err)
	}
	if _, err := svc.Create(ctx, product.CreateInput{
		SellerID: otherSellerID, Title: "From seller B",
		Price: 200, CurrencyCode: "USD",
	}); err != nil {
		t.Fatalf("create B: %v", err)
	}

	page, err := svc.Search(ctx, "", "", sellerID, "", 30, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(page.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(page.Results))
	}
	if page.Results[0].SellerID != sellerID {
		t.Errorf("seller_id = %q, want %q", page.Results[0].SellerID, sellerID)
	}
}

func TestSearch_NoMatch(t *testing.T) {
	svc, _, _ := setup(t)
	page, err := svc.Search(context.Background(), "xyzzy_nothing_here_42", "", "", "", 30, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(page.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(page.Results))
	}
}

func TestSearch_Pagination(t *testing.T) {
	svc, _, sellerID := setup(t)
	ctx := context.Background()

	prefix := fmt.Sprintf("pgt%d", time.Now().UnixNano()%100_000)
	for i := 0; i < 3; i++ {
		if _, err := svc.Create(ctx, product.CreateInput{
			SellerID: sellerID, Title: fmt.Sprintf("%s_%d", prefix, i),
			Price: 100, CurrencyCode: "USD",
		}); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}

	page1, err := svc.Search(ctx, prefix, "", "", "", 2, 0)
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(page1.Results) != 2 {
		t.Errorf("page1 len = %d, want 2", len(page1.Results))
	}
	page2, err := svc.Search(ctx, prefix, "", "", "", 2, 2)
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(page2.Results) != 1 {
		t.Errorf("page2 len = %d, want 1", len(page2.Results))
	}
	if page1.Results[0].ID == page2.Results[0].ID {
		t.Error("pages overlap")
	}
	if page1.Total < 3 {
		t.Errorf("total = %d, want >= 3", page1.Total)
	}
}

// ---- helpers ----

func createSubcategory(t *testing.T, pool *pgxpool.Pool, slugPrefix, parentID string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `
		INSERT INTO categories (slug, name, parent_id) VALUES ($1, $1, $2) RETURNING id::text
	`, fmt.Sprintf("%s-%d", slugPrefix, time.Now().UnixNano()%1_000_000_000), parentID).Scan(&id)
	if err != nil {
		t.Fatalf("create subcategory: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM products WHERE category_id=$1", id)
		_, _ = pool.Exec(context.Background(), "DELETE FROM categories WHERE id=$1", id)
	})
	return id
}

func createOtherSeller(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	suffix := time.Now().UnixNano() % 1_000_000_000
	var id string
	err := pool.QueryRow(context.Background(), `
		INSERT INTO users (email, password_hash, username, preferred_currency)
		VALUES ($1, 'x', $2, 'USD') RETURNING id::text
	`,
		fmt.Sprintf("os_%d@example.com", suffix),
		fmt.Sprintf("os_%d", suffix%1_000_000),
	).Scan(&id)
	if err != nil {
		t.Fatalf("create other seller: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM products WHERE seller_id=$1", id)
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id=$1", id)
	})
	return id
}
