package wallet

func ptrString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
