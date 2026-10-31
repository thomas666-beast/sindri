package wallet

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

func encodeTxCursor(t time.Time, id string) string {
	raw := fmt.Sprintf("%d|%s", t.UnixNano(), id)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeTxCursor(s string) (time.Time, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, "", errors.New("invalid cursor")
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, "", errors.New("invalid cursor")
	}
	var ns int64
	if _, err := fmt.Sscan(parts[0], &ns); err != nil {
		return time.Time{}, "", errors.New("invalid cursor")
	}
	if _, err := uuid.Parse(parts[1]); err != nil {
		return time.Time{}, "", errors.New("invalid cursor")
	}
	return time.Unix(0, ns), parts[1], nil
}
