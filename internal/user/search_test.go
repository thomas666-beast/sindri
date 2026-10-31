package user_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thomas666-beast/sindri/internal/db/dbgen"
	"github.com/thomas666-beast/sindri/internal/user"
)

func searchSetup(t *testing.T) (*user.Service, *pgxpool.Pool) {
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
	return user.NewService(dbgen.New(pool)), pool
}

func makeUser(t *testing.T, svc *user.Service, pool *pgxpool.Pool, prefix string) string {
	t.Helper()
	suffix := time.Now().UnixNano()%1_000_000_000 + int64(len(prefix))
	u, err := svc.Register(context.Background(), user.RegisterInput{
		Email:    fmt.Sprintf("%s_%d@example.com", prefix, suffix),
		Username: fmt.Sprintf("%s_%d", prefix, suffix%1_000_000),
		Password: "hunter2hunter2",
	})
	if err != nil {
		t.Fatalf("register %s: %v", prefix, err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM user_profiles WHERE user_id=$1", u.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id=$1", u.ID)
	})
	return u.ID
}

func setProfile(t *testing.T, pool *pgxpool.Pool, userID, bio string, skills, interests []string) {
	t.Helper()
	if skills == nil {
		skills = []string{}
	}
	if interests == nil {
		interests = []string{}
	}
	_, err := pool.Exec(context.Background(), `
		INSERT INTO user_profiles (user_id, display_name, bio, skills, interests)
		VALUES ($1, 'x', $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE
		SET bio = EXCLUDED.bio, skills = EXCLUDED.skills, interests = EXCLUDED.interests
	`, userID, bio, skills, interests)
	if err != nil {
		t.Fatalf("set profile: %v", err)
	}
}

func TestSearch_ByUsername(t *testing.T) {
	svc, pool := searchSetup(t)
	ctx := context.Background()

	uniqName := fmt.Sprintf("zebrafish_%d", time.Now().UnixNano()%1_000_000)
	u, err := svc.Register(ctx, user.RegisterInput{
		Email:    uniqName + "@example.com",
		Username: uniqName,
		Password: "hunter2hunter2",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id=$1", u.ID)
	})

	page, err := svc.Search(ctx, "zebrafish", 30, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	found := false
	for _, r := range page.Results {
		if r.ID == u.ID {
			found = true
			if r.Username != uniqName {
				t.Errorf("username = %q", r.Username)
			}
		}
	}
	if !found {
		t.Fatal("user not found by username search")
	}
}

func TestSearch_ByBio(t *testing.T) {
	svc, pool := searchSetup(t)
	ctx := context.Background()

	uid := makeUser(t, svc, pool, "biomatch")
	setProfile(t, pool, uid, "I build distributed systems in Go", nil, nil)

	page, err := svc.Search(ctx, "distributed systems", 30, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	found := false
	for _, r := range page.Results {
		if r.ID == uid {
			found = true
			if r.Bio != "I build distributed systems in Go" {
				t.Errorf("bio = %q", r.Bio)
			}
		}
	}
	if !found {
		t.Fatal("user not found by bio")
	}
}

func TestSearch_BySkill(t *testing.T) {
	svc, pool := searchSetup(t)
	ctx := context.Background()

	uid := makeUser(t, svc, pool, "skillmatch")
	setProfile(t, pool, uid, "", []string{"golang", "postgres", "kubernetes"}, nil)

	page, err := svc.Search(ctx, "kuber", 30, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	found := false
	for _, r := range page.Results {
		if r.ID == uid {
			found = true
		}
	}
	if !found {
		t.Fatal("user not found by skill substring 'kuber'")
	}
}

func TestSearch_ByInterest(t *testing.T) {
	svc, pool := searchSetup(t)
	ctx := context.Background()

	uid := makeUser(t, svc, pool, "intmatch")
	setProfile(t, pool, uid, "", nil, []string{"woodworking", "coffee", "mechanical keyboards"})

	page, err := svc.Search(ctx, "keyboard", 30, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	found := false
	for _, r := range page.Results {
		if r.ID == uid {
			found = true
		}
	}
	if !found {
		t.Fatal("user not found by interest")
	}
}

func TestSearch_CaseInsensitive(t *testing.T) {
	svc, pool := searchSetup(t)
	ctx := context.Background()

	uniqName := fmt.Sprintf("UPPER_%d", time.Now().UnixNano()%1_000_000)
	u, err := svc.Register(ctx, user.RegisterInput{
		Email:    uniqName + "@example.com",
		Username: uniqName,
		Password: "hunter2hunter2",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id=$1", u.ID)
	})

	page, err := svc.Search(ctx, "upper", 30, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	found := false
	for _, r := range page.Results {
		if r.ID == u.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("case-insensitive match failed")
	}
}

func TestSearch_ShortQuery(t *testing.T) {
	svc, _ := searchSetup(t)
	_, err := svc.Search(context.Background(), "a", 30, 0)
	if !errors.Is(err, user.ErrSearchQueryTooShort) {
		t.Fatalf("err = %v, want ErrSearchQueryTooShort", err)
	}
}

func TestSearch_NoMatch(t *testing.T) {
	svc, _ := searchSetup(t)
	page, err := svc.Search(context.Background(), "xyzzy_no_such_thing_42", 30, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(page.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(page.Results))
	}
}

func TestSearch_FillsDefaultDisplayName(t *testing.T) {
	svc, pool := searchSetup(t)
	ctx := context.Background()

	uniqName := fmt.Sprintf("profiledefault_%d", time.Now().UnixNano()%1_000_000)
	u, err := svc.Register(ctx, user.RegisterInput{
		Email:    uniqName + "@example.com",
		Username: uniqName,
		Password: "hunter2hunter2",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id=$1", u.ID)
	})

	// No profile row — display_name should fall back to username.
	page, err := svc.Search(ctx, "profiledefault", 30, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	found := false
	for _, r := range page.Results {
		if r.ID == u.ID {
			found = true
			if r.DisplayName != uniqName {
				t.Errorf("display_name = %q, want %q (username fallback)", r.DisplayName, uniqName)
			}
			if r.Bio != "" {
				t.Errorf("bio = %q, want empty", r.Bio)
			}
			if len(r.Skills) != 0 {
				t.Errorf("skills = %v, want empty", r.Skills)
			}
		}
	}
	if !found {
		t.Fatal("user not found")
	}
}

func TestSearch_Pagination(t *testing.T) {
	svc, pool := searchSetup(t)
	ctx := context.Background()

	prefix := fmt.Sprintf("pgtst%d", time.Now().UnixNano()%100_000)
	for i := 0; i < 3; i++ {
		_, err := svc.Register(ctx, user.RegisterInput{
			Email:    fmt.Sprintf("%s_%d@example.com", prefix, i),
			Username: fmt.Sprintf("%s_%d", prefix, i),
			Password: "hunter2hunter2",
		})
		if err != nil {
			t.Fatalf("register %d: %v", i, err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			"DELETE FROM users WHERE username LIKE $1", prefix+"%")
	})

	page1, err := svc.Search(ctx, prefix, 2, 0)
	if err != nil {
		t.Fatalf("search page1: %v", err)
	}
	if len(page1.Results) != 2 {
		t.Errorf("page1 len = %d, want 2", len(page1.Results))
	}
	page2, err := svc.Search(ctx, prefix, 2, 2)
	if err != nil {
		t.Fatalf("search page2: %v", err)
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
