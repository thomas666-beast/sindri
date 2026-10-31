package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thomas666-beast/sindri/internal/db/dbgen"
	"github.com/thomas666-beast/sindri/internal/order"
)

type ReleaseWorker struct {
	pool     *pgxpool.Pool
	orderSvc *order.Service
}

func NewReleaseWorker(pool *pgxpool.Pool, orderSvc *order.Service) *ReleaseWorker {
	return &ReleaseWorker{pool: pool, orderSvc: orderSvc}
}

// Run blocks forever, checking for stale orders every 5 minutes.
func (w *ReleaseWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	slog.Info("auto-release worker started")

	// Run once immediately on startup, then on the ticker
	w.processStaleOrders(ctx)

	for {
		select {
		case <-ctx.Done():
			slog.Info("auto-release worker stopped")
			return
		case <-ticker.C:
			w.processStaleOrders(ctx)
		}
	}
}

func (w *ReleaseWorker) processStaleOrders(ctx context.Context) {
	q := dbgen.New(w.pool)
	
	// Find up to 100 stale orders
	orders, err := q.FindStaleHeldOrders(ctx, 100)
	if err != nil {
		slog.Error("worker: failed to fetch stale orders", "error", err)
		return
	}

	if len(orders) == 0 {
		return
	}

	slog.Info("worker: found stale orders to auto-release", "count", len(orders))

	for _, o := range orders {
		// AutoRelease handles the transaction, escrow release, and notifications
		_, err := w.orderSvc.AutoRelease(ctx, o.ID)
		if err != nil {
			slog.Error("worker: failed to auto-release order", "order_id", o.ID, "error", err)
		} else {
			slog.Info("worker: successfully auto-released order", "order_id", o.ID)
		}
	}
}
