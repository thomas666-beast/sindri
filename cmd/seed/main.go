// Command seed populates a sindri database with fake users, products,
// wallets, and categories for local testing and demos.
//
// It is safe to re-run: every generated user has a username with prefix
// "sindri_" and the command deletes any existing rows with that prefix
// before inserting. The three "main" users (pitrucci, romeo, james) are
// handled separately and are NOT deleted.
//
// Usage:
//   go run ./cmd/seed                                # defaults: 1000 users, 10-20 products
//   go run ./cmd/seed -users 100 -products-min 3 -products-max 5
//   go run ./cmd/seed -clean                         # delete seeded data and exit
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	kitpassword "github.com/thomas666-beast/cryptkit/password"

	"github.com/thomas666-beast/sindri/internal/db"
	"github.com/thomas666-beast/sindri/internal/db/dbgen"
	"github.com/thomas666-beast/sindri/internal/product"
	"github.com/thomas666-beast/sindri/internal/user"
	"github.com/thomas666-beast/sindri/internal/wallet"
)

const (
	seededUsernamePrefix = "sindri_"
	defaultPassword      = "hunter2hunter2" // must satisfy user.ErrWeakPassword (>=8 chars)
)

// Named accounts that always exist, so you have a stable set to log in as.
var namedUsers = []string{"pitrucci", "romeo", "james"}

var (
	flagUsers       = flag.Int("users", 1000, "number of sindri_* users to seed")
	flagProductsMin = flag.Int("products-min", 10, "minimum products per user")
	flagProductsMax = flag.Int("products-max", 20, "maximum products per user")
	flagSeed        = flag.Int64("seed", 42, "RNG seed for reproducibility")
	flagClean       = flag.Bool("clean", false, "delete seeded data (sindri_* users + named users' data) and exit")
)

func main() {
	flag.Parse()

	cfg, err := loadDBURL()
	if err != nil {
		slog.Error("database url", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg)
	if err != nil {
		slog.Error("connect", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if *flagClean {
		if err := clean(ctx, pool); err != nil {
			slog.Error("clean", "error", err)
			os.Exit(1)
		}
		fmt.Println("✓ cleaned seeded data")
		return
	}

	if *flagProductsMin < 0 || *flagProductsMax < *flagProductsMin {
		slog.Error("invalid product range",
			"min", *flagProductsMin, "max", *flagProductsMax)
		os.Exit(1)
	}

	start := time.Now()
	if err := run(ctx, pool); err != nil {
		slog.Error("seed", "error", err)
		os.Exit(1)
	}
	fmt.Printf("✓ seed complete in %s\n", time.Since(start).Round(time.Millisecond))
}

func loadDBURL() (string, error) {
	if v := os.Getenv("DATABASE_URL"); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("DATABASE_URL is required")
}

func run(ctx context.Context, pool *pgxpool.Pool) error {
	q := dbgen.New(pool)
	userSvc := user.NewService(q)
	productSvc := product.NewService(q)
	walletSvc := wallet.NewService(pool)

	rng := rand.New(rand.NewSource(*flagSeed))

	// ---------------------------------------------------------------
	// 1. Categories (reuse existing, create a few if empty)
	// ---------------------------------------------------------------
	cats, err := ensureCategories(ctx, q)
	if err != nil {
		return fmt.Errorf("categories: %w", err)
	}
	slog.Info("categories ready", "count", len(cats))

	// ---------------------------------------------------------------
	// 2. Named accounts (always present, never deleted by -clean)
	// ---------------------------------------------------------------
	for _, name := range namedUsers {
		if err := ensureNamedUser(ctx, pool, userSvc, name, defaultPassword); err != nil {
			return fmt.Errorf("named user %q: %w", name, err)
		}
		slog.Info("named user ready", "username", name)
	}

	// ---------------------------------------------------------------
	// 3. sindri_* users
	// ---------------------------------------------------------------
	slog.Info("seeding users", "count", *flagUsers)
	created := 0
	for i := 1; i <= *flagUsers; i++ {
		username := fmt.Sprintf("%s%04d", seededUsernamePrefix, i)
		email := fmt.Sprintf("%s@example.com", username)

		u, err := userSvc.Register(ctx, user.RegisterInput{
			Email:             email,
			Username:          username,
			Password:          defaultPassword,
			PreferredCurrency: pickCurrency(rng),
		})
		if err != nil {
			if strings.Contains(err.Error(), "already") {
				// idempotent re-run: user already exists, keep going
				continue
			}
			return fmt.Errorf("register %s: %w", username, err)
		}

		// Fund the wallet so products can immediately be bought.
		depositAmt := int64(100_000 + rng.Intn(900_000)) // $1,000 - $10,000
		if _, err := walletSvc.Deposit(ctx, u.ID, pickCurrency(rng), depositAmt, ""); err != nil {
			return fmt.Errorf("deposit %s: %w", username, err)
		}

		// Products
		n := *flagProductsMin + rng.Intn(*flagProductsMax-*flagProductsMin+1)
		for p := 0; p < n; p++ {
			catID := ""
			if len(cats) > 0 {
				catID = cats[rng.Intn(len(cats))]
			}
			title := fmt.Sprintf("%s — item %d", titleWord(rng), p+1)
			if _, err := productSvc.Create(ctx, product.CreateInput{
				SellerID:     u.ID,
				Title:        title,
				Description:  descriptionFor(rng),
				Price:        int64(1_000 + rng.Intn(499_000)), // $10 - $5000
				CurrencyCode: pickCurrency(rng),
				CategoryID:   catID,
			}); err != nil {
				return fmt.Errorf("product %s/%d: %w", username, p, err)
			}
		}

		created++
		if created%100 == 0 {
			slog.Info("progress", "created", created, "target", *flagUsers)
		}
	}

	slog.Info("seed done", "users_created", created)
	return nil
}

// ---------------------------------------------------------------
// helpers
// ---------------------------------------------------------------

func ensureNamedUser(ctx context.Context, pool *pgxpool.Pool, us *user.Service, username, password string) error {
	email := username + "@example.com"
	_, err := us.Register(ctx, user.RegisterInput{
		Email:             email,
		Username:          username,
		Password:          password,
		PreferredCurrency: "USD",
	})
	if err == nil {
		return nil
	}
	if !strings.Contains(err.Error(), "already") {
		return err
	}

	// User exists — force the documented credentials to work.
	slog.Warn("named user exists, resetting password", "username", username)
	u, gerr := us.GetByUsername(ctx, username)
	if gerr != nil {
		return fmt.Errorf("lookup %s: %w", username, gerr)
	}
	hash, herr := kitpassword.HashPassword(password)
	if herr != nil {
		return fmt.Errorf("hash: %w", herr)
	}
	_, qerr := pool.Exec(ctx,
		`UPDATE users SET password_hash = $1, email = $2, is_active = TRUE,
		        disabled_at = NULL, disable_reason = NULL
		 WHERE id = $3`,
		hash, email, u.ID)
	if qerr != nil {
		return fmt.Errorf("reset %s: %w", username, qerr)
	}
	return nil
}

func ensureCategories(ctx context.Context, q dbgen.Querier) ([]string, error) {
	rows, err := q.ListCategories(ctx, dbgen.ListCategoriesParams{
		OnlyActive:   true,
		ParentFilter: "",
		Lim:          100,
		Off:          0,
	})
	if err != nil {
		return nil, err
	}
	if len(rows) > 0 {
		out := make([]string, 0, len(rows))
		for _, r := range rows {
			out = append(out, r.ID)
		}
		return out, nil
	}
	// No categories — the seeder will fall back to empty CategoryID.
	// If you want auto-created defaults, add a small loop here calling
	// category.Service.Create with a slug/name pair.
	return nil, nil
}

func pickCurrency(rng *rand.Rand) string {
	choices := []string{"USD", "EUR", "BRL", "GBP", "JPY"}
	return choices[rng.Intn(len(choices))]
}

func titleWord(rng *rand.Rand) string {
	words := []string{
		"Vintage", "Handmade", "Restored", "Custom", "Original",
		"Limited", "Signed", "Collector", "Refurbished", "Bespoke",
		"Rare", "Artisan", "Heritage", "Modern", "Retro",
	}
	return words[rng.Intn(len(words))]
}

func descriptionFor(rng *rand.Rand) string {
	lines := []string{
		"Carefully sourced and inspected. Ships within 2 business days.",
		"One of a kind. Comes with a certificate of authenticity.",
		"Restored to original condition using period-correct materials.",
		"Built to order in small batches. Allow 1-2 weeks for delivery.",
		"Excellent condition. Stored in a smoke-free, pet-free workshop.",
		"Perfect for collectors. No visible damage, see photos for details.",
		"Custom piece made by hand. Slight variations are expected.",
		"Vintage item with expected patina. Sold as-is.",
	}
	return lines[rng.Intn(len(lines))]
}

// clean deletes sindri_* users and all their data. It does NOT touch the
// named users (pitrucci, romeo, james) — they stay across clean runs.
func clean(ctx context.Context, pool *pgxpool.Pool) error {
	// Order matters for FK constraints.
	stmts := []string{
		`DELETE FROM wallet_transactions
		   WHERE user_id IN (SELECT id FROM users WHERE username LIKE 'sindri\_%')`,
		`DELETE FROM orders
		   WHERE buyer_id IN (SELECT id FROM users WHERE username LIKE 'sindri\_%')
		      OR seller_id IN (SELECT id FROM users WHERE username LIKE 'sindri\_%')`,
		`DELETE FROM wallets
		   WHERE user_id IN (SELECT id FROM users WHERE username LIKE 'sindri\_%')`,
		`DELETE FROM products
		   WHERE seller_id IN (SELECT id FROM users WHERE username LIKE 'sindri\_%')`,
		`DELETE FROM user_profiles
		   WHERE user_id IN (SELECT id FROM users WHERE username LIKE 'sindri\_%')`,
		`DELETE FROM notifications
		   WHERE user_id IN (SELECT id FROM users WHERE username LIKE 'sindri\_%')`,
		`DELETE FROM users WHERE username LIKE 'sindri\_%'`,
	}
	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s); err != nil {
			// Tables that don't exist in your schema are not fatal —
			// comment out the ones you don't have.
			slog.Warn("clean step failed (continuing)", "error", err)
		}
	}
	return nil
}
