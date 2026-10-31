import { api } from '../api';

export interface Balance {
  currency_code: string;
  available: number;
  held: number;
}

export interface Currency {
  code: string;
  name: string;
  symbol: string;
  decimal_digits: number;
}

export async function listWallets(): Promise<Balance[]> {
  const { data } = await api.get<{ data: Balance[] }>('/wallet');
  return data.data;
}

export async function listCurrencies(): Promise<Currency[]> {
  const { data } = await api.get<{ data: Currency[] }>('/currencies');
  return data.data;
}

export async function deposit(currency: string, amount: number): Promise<Balance> {
  const { data } = await api.post<{ data: Balance }>('/wallet/deposit', {
    currency,
    amount,
  });
  return data.data;
}

export interface Transaction {
  id: string;
  type: string;
  direction: 'credit' | 'debit';
  amount: number;
  currency_code: string;
  balance_after: number;
  order_id?: string;
  status: string;
  metadata: string;
  created_at_unix: number;
}

export interface TransactionPage {
  transactions: Transaction[];
  next_cursor?: string;
  has_more: boolean;
}

export async function listTransactions(
  currency = '',
  cursor = '',
  limit = 30
): Promise<TransactionPage> {
  const { data } = await api.get<{ data: TransactionPage }>(
    '/wallet/transactions',
    { params: { currency, cursor, limit } }
  );
  return data.data;
}
