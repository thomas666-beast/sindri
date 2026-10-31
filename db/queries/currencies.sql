-- name: ListCurrencies :many
SELECT code, name, symbol, decimal_digits, is_active
FROM currencies
WHERE is_active = TRUE
ORDER BY code;

-- name: GetCurrency :one
SELECT code, name, symbol, decimal_digits, is_active
FROM currencies
WHERE code = $1;

-- name: CountCurrencies :one
SELECT count(*) FROM currencies;
