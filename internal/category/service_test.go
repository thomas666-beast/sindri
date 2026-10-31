package category_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thomas666-beast/sindri/internal/category"
	"github.com/thomas666-beast/sindri/internal/db/dbgen"
)

func setup(t *testing.T) (*category.Service, *pgxpool.Pool) {
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

	t.Cleanup(func() {
		// Delete children first, then parents.
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM categories WHERE parent_id IS NOT NULL AND slug LIKE 'test-%'`)
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM categories WHERE slug LIKE 'test-%'`)
	})

	return category.NewService(dbgen.New(pool)), pool
}

func uniq(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()%1_000_000_000)
}

func TestCreate_TopLevel(t *testing.T) {
	svc, _ := setup(t)
	c, err := svc.Create(context.Background(), category.CreateInput{
		Slug: uniq("test-top"), Name: "Top",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if c.ParentID != "" {
		t.Errorf("parent_id = %q, want empty", c.ParentID)
	}
}

func TestCreate_Subcategory(t *testing.T) {
	svc, _ := setup(t)
	ctx := context.Background()
	parent, _ := svc.Create(ctx, category.CreateInput{Slug: uniq("test-par"), Name: "Parent"})
	child, err := svc.Create(ctx, category.CreateInput{
		Slug: uniq("test-chi"), Name: "Child", ParentID: parent.ID,
	})
	if err != nil {
		t.Fatalf("create child: %v", err)
	}
	if child.ParentID != parent.ID {
		t.Errorf("parent_id = %q, want %q", child.ParentID, parent.ID)
	}
}

func TestCreate_ParentNotFound(t *testing.T) {
	svc, _ := setup(t)
	_, err := svc.Create(context.Background(), category.CreateInput{
		Slug: uniq("test-x"), Name: "X",
		ParentID: "00000000-0000-0000-0000-000000000000",
	})
	if !errors.Is(err, category.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestCreate_ParentIsItselfSubcategory(t *testing.T) {
	svc, _ := setup(t)
	ctx := context.Background()
	parent, _ := svc.Create(ctx, category.CreateInput{Slug: uniq("test-p1"), Name: "P1"})
	child, _ := svc.Create(ctx, category.CreateInput{
		Slug: uniq("test-c1"), Name: "C1", ParentID: parent.ID,
	})
	// Try to make another category whose parent is the child (grandchild)
	_, err := svc.Create(ctx, category.CreateInput{
		Slug: uniq("test-gc"), Name: "GC", ParentID: child.ID,
	})
	if !errors.Is(err, category.ErrParentHasParent) {
		t.Fatalf("err = %v, want ErrParentHasParent", err)
	}
}

func TestList_TopLevelOnly(t *testing.T) {
	svc, _ := setup(t)
	ctx := context.Background()
	parent, _ := svc.Create(ctx, category.CreateInput{Slug: uniq("test-lp"), Name: "LP"})
	svc.Create(ctx, category.CreateInput{Slug: uniq("test-lc"), Name: "LC", ParentID: parent.ID})

	top, err := svc.List(ctx, false, "top", 100, 0)
	if err != nil {
		t.Fatalf("list top: %v", err)
	}
	// Among test-* categories, only the parent should appear as top
	for _, c := range top {
		if c.ParentID != "" && len(c.Slug) > 4 && c.Slug[:4] == "test" {
			t.Errorf("subcategory %q appeared in top-level list", c.Slug)
		}
	}
}

func TestList_ChildrenOnly(t *testing.T) {
	svc, _ := setup(t)
	ctx := context.Background()
	parent, _ := svc.Create(ctx, category.CreateInput{Slug: uniq("test-cp"), Name: "CP"})
	child, _ := svc.Create(ctx, category.CreateInput{Slug: uniq("test-cc"), Name: "CC", ParentID: parent.ID})

	children, err := svc.List(ctx, false, "children", 100, 0)
	if err != nil {
		t.Fatalf("list children: %v", err)
	}
	found := false
	for _, c := range children {
		if c.ID == child.ID {
			found = true
		}
	}
	if !found {
		t.Error("child not in children list")
	}
}

func TestListChildren_ByParentID(t *testing.T) {
	svc, _ := setup(t)
	ctx := context.Background()
	parent, _ := svc.Create(ctx, category.CreateInput{Slug: uniq("test-lcp"), Name: "LCP"})
	svc.Create(ctx, category.CreateInput{Slug: uniq("test-lcc1"), Name: "LCC1", ParentID: parent.ID})
	svc.Create(ctx, category.CreateInput{Slug: uniq("test-lcc2"), Name: "LCC2", ParentID: parent.ID})

	kids, err := svc.ListChildren(ctx, parent.ID, false, 100, 0)
	if err != nil {
		t.Fatalf("list children: %v", err)
	}
	if len(kids) != 2 {
		t.Errorf("expected 2 children, got %d", len(kids))
	}
}

func TestDelete_WithChildren_Refused(t *testing.T) {
	svc, _ := setup(t)
	ctx := context.Background()
	parent, _ := svc.Create(ctx, category.CreateInput{Slug: uniq("test-dp"), Name: "DP"})
	svc.Create(ctx, category.CreateInput{Slug: uniq("test-dc"), Name: "DC", ParentID: parent.ID})

	err := svc.Delete(ctx, parent.ID)
	if !errors.Is(err, category.ErrHasChildren) {
		t.Fatalf("err = %v, want ErrHasChildren", err)
	}

	// Still present after failed delete
	if _, err := svc.Get(ctx, parent.ID); err != nil {
		t.Fatalf("parent should still exist: %v", err)
	}
}

func TestDelete_ChildThenParent(t *testing.T) {
	svc, _ := setup(t)
	ctx := context.Background()
	parent, _ := svc.Create(ctx, category.CreateInput{Slug: uniq("test-zp"), Name: "ZP"})
	child, _ := svc.Create(ctx, category.CreateInput{Slug: uniq("test-zc"), Name: "ZC", ParentID: parent.ID})

	if err := svc.Delete(ctx, child.ID); err != nil {
		t.Fatalf("delete child: %v", err)
	}
	if err := svc.Delete(ctx, parent.ID); err != nil {
		t.Fatalf("delete parent after child: %v", err)
	}
}

func TestGet_StillWorks(t *testing.T) {
	svc, _ := setup(t)
	c, _ := svc.Create(context.Background(), category.CreateInput{Slug: uniq("test-g"), Name: "G"})
	got, err := svc.Get(context.Background(), c.ID)
	if err != nil { t.Fatalf("get: %v", err) }
	if got.ID != c.ID { t.Error("id mismatch") }
}

func TestUpdate_PreservesParentID(t *testing.T) {
	svc, _ := setup(t)
	ctx := context.Background()
	parent, _ := svc.Create(ctx, category.CreateInput{Slug: uniq("test-up"), Name: "UP"})
	child, _ := svc.Create(ctx, category.CreateInput{Slug: uniq("test-uc"), Name: "UC", ParentID: parent.ID})

	upd, err := svc.Update(ctx, category.UpdateInput{
		ID: child.ID, Slug: child.Slug, Name: "UC-Renamed",
		Description: "", SortOrder: 0, IsActive: true,
	})
	if err != nil { t.Fatalf("update: %v", err) }
	if upd.ParentID != parent.ID {
		t.Errorf("parent_id = %q, want %q (should be preserved)", upd.ParentID, parent.ID)
	}
}
