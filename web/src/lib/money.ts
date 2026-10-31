import type { Currency } from '../api/wallet';
import { api } from '../api';

let cachedCurrencies: Currency[] | null = null;
let pendingCurrencies: Promise<Currency[]> | null = null;

export async function getCurrencies(): Promise<Currency[]> {
  if (cachedCurrencies) return cachedCurrencies;
  if (pendingCurrencies) return pendingCurrencies;

  pendingCurrencies = api
    .get<{ data: Currency[] }>('/currencies')
    .then((res) => {
      cachedCurrencies = res.data.data;
      pendingCurrencies = null;
      return cachedCurrencies;
    })
    .catch((err) => {
      pendingCurrencies = null;
      throw err;
    });

  return pendingCurrencies;
}

// digitsFor returns the number of decimal places for a currency.
//
// Two states are distinguished:
//   - currencies.length === 0 → the list hasn't loaded yet. We return 2
//     (the ISO-4217 majority) so computed previews don't crash during the
//     first paint. The value will be correct once the list arrives.
//   - currencies.length > 0 but code is missing → a real bug: the backend
//     sent us a currency code we don't know. Throw, so the bug surfaces
//     instead of silently rendering the wrong decimal places.
export function digitsFor(currencies: Currency[], code: string): number {
  if (currencies.length === 0) {
    return 2;
  }
  const c = currencies.find((x) => x.code === code);
  if (!c) {
    throw new Error(`money: unknown currency ${code} (backend sent a code we don't know)`);
  }
  return c.decimal_digits;
}

// symbolFor returns the currency's display symbol. Empty list → return the
// code itself (visible, not wrong-looking). Non-empty list missing the code
// → also return the code (a display fallback, not a data fallback — the
// number itself comes from digitsFor, which throws on real bugs).
export function symbolFor(currencies: Currency[], code: string): string {
  return currencies.find((c) => c.code === code)?.symbol ?? code;
}

export function fmtAmount(
  currencies: Currency[],
  cents: number,
  code: string,
): string {
  const digits = digitsFor(currencies, code);
  return (cents / Math.pow(10, digits)).toFixed(digits);
}

export function fmtWithSymbol(
  currencies: Currency[],
  cents: number,
  code: string,
): string {
  return `${symbolFor(currencies, code)} ${fmtAmount(currencies, cents, code)}`;
}

// Parse user-typed dollars into integer cents.
//   "50.25" with 2 digits → 5025
//   "50"    with 2 digits → 5000
//   "50,25" with 2 digits → 5025 (comma accepted)
//   "5000"  with 0 digits (JPY) → 5000
export function parseAmount(
  input: string | number | null,
  decimalDigits: number,
): number | null {
  if (input == null || input === '') return null;
  const raw = String(input).trim().replace(/,/g, '.');
  if (raw === '') return null;
  const num = Number(raw);
  if (!isFinite(num) || num < 0) return null;
  const scale = Math.pow(10, decimalDigits);
  return Math.round(num * scale);
}

// Convert integer cents back to a decimal string for editing.
//   5025 with 2 digits → "50.25"
//   5000 with 0 digits → "5000"
export function formatForInput(cents: number, decimalDigits: number): string {
  if (decimalDigits === 0) return String(cents);
  const scale = Math.pow(10, decimalDigits);
  return (cents / scale).toFixed(decimalDigits);
}
