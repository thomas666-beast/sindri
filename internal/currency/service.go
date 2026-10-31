package currency

import (
	"context"
	"fmt"

	"github.com/thomas666-beast/sindri/internal/db/dbgen"
)

type Currency struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	Symbol        string `json:"symbol"`
	DecimalDigits int16  `json:"decimal_digits"`
}

type Service struct {
	q dbgen.Querier
}

func NewService(q dbgen.Querier) *Service {
	return &Service{q: q}
}

func (s *Service) List(ctx context.Context) ([]Currency, error) {
	rows, err := s.q.ListCurrencies(ctx)
	if err != nil {
		return nil, fmt.Errorf("list currencies: %w", err)
	}

	out := make([]Currency, 0, len(rows))
	for _, r := range rows {
		out = append(out, Currency{
			Code:          r.Code,
			Name:          r.Name,
			Symbol:        r.Symbol,
			DecimalDigits: r.DecimalDigits,
		})
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, code string) (*Currency, error) {
	row, err := s.q.GetCurrency(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("get currency %q: %w", code, err)
	}
	return &Currency{
		Code:          row.Code,
		Name:          row.Name,
		Symbol:        row.Symbol,
		DecimalDigits: row.DecimalDigits,
	}, nil
}
