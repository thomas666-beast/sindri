import { api } from '../api';
import type { Order } from './orders';

export interface InvoiceItem extends Order {
  cancel_reason?: string;
  cancelled_by_id?: string;
}

export interface CreateInvoiceInput {
  buyer_username: string;
  title: string;
  description: string;
  amount: number;
  currency: string;
}

export interface ListParams {
  role?: '' | 'buyer' | 'seller';
  status?: '' | 'pending' | 'held' | 'completed' | 'refunded' | 'cancelled';
  id?: string; // Add this
  limit?: number;
  offset?: number;
}

export async function create(input: CreateInvoiceInput): Promise<Order> {
  const { data } = await api.post<{ data: Order }>('/invoices', input);
  return data.data;
}

export async function list(params: ListParams = {}): Promise<{ data: InvoiceItem[]; total: number }> {
  const { data } = await api.get<{ data: InvoiceItem[]; total: number }>('/invoices', {
    params: {
      role: params.role ?? '',
      status: params.status ?? '',
      id: params.id ?? '', // Add this
      limit: params.limit ?? 30,
      offset: params.offset ?? 0,
    },
  });
  return data;
}

export async function get(id: string): Promise<InvoiceItem> {
  const { data } = await api.get<{ data: InvoiceItem }>(`/invoices/${id}`);
  return data.data;
}

export async function pay(id: string): Promise<Order> {
  const { data } = await api.post<{ data: Order }>(`/invoices/${id}/pay`);
  return data.data;
}

export async function cancel(id: string, reason: string): Promise<InvoiceItem> {
  const { data } = await api.post<{ data: InvoiceItem }>(`/invoices/${id}/cancel`, {
    reason,
  });
  return data.data;
}
