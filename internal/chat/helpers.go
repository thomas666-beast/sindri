package chat

import "github.com/jackc/pgx/v5/pgtype"

func pgText(s *string) *string { return s }

func pgtypeText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}
