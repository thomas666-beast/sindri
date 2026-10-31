import { api } from '../api';

export interface Order {
  id: string;
  buyer_id: string;
  buyer_username: string;
  seller_id: string;
  seller_username: string;
  product_id?: string;
  invoice_title?: string;
  invoice_description?: string;
  currency_code: string;
  amount: number;
  commission_amount: number;
  commission_percent: number;
  status: 'pending' | 'held' | 'delivered' | 'disputed' | 'completed' | 'refunded' | 'cancelled';
  my_role?: 'buyer' | 'seller';
  auto_release_at_unix?: number;
  released_at_unix?: number;
  released_by?: string;
  created_at_unix: number;

  dispute_reason?: string;
  dispute_opened_by_username?: string;
}

export interface OrderPage {
  orders: Order[];
  next_cursor?: string;
  has_more: boolean;
}

export async function create(productId: string): Promise<Order> {
  const { data } = await api.post<{ data: Order }>('/orders', {
    product_id: productId,
  });
  return data.data;
}

export async function list(
  role: '' | 'buyer' | 'seller' = '',
  cursor = '',
  limit = 20,
  id = '' // Add this
): Promise<OrderPage> {
  const { data } = await api.get<{ data: OrderPage }>('/orders', {
    params: { role, cursor, limit, id }, // Add id here
  });
  return data.data;
}

export async function get(id: string): Promise<Order> {
  const { data } = await api.get<{ data: Order }>(`/orders/${id}`);
  return data.data;
}

export async function confirm(id: string): Promise<Order> {
  const { data } = await api.post<{ data: Order }>(`/orders/${id}/confirm`);
  return data.data;
}

export async function dispute(id: string, reason: string): Promise<Order> {
  const { data } = await api.post<{ data: Order }>(`/orders/${id}/dispute`, {
    reason,
  });
  return data.data;
}

export interface Deliverable {
  id: string;
  filename: string;
  content_type: string;
  size_bytes: number;
}

export async function listDeliverables(orderId: string): Promise<Deliverable[]> {
  const { data } = await api.get<{ data: Deliverable[] }>(`/orders/${orderId}/deliverables`);
  return data.data;
}

export async function uploadDeliverable(orderId: string, file: File): Promise<void> {
  const form = new FormData();
  form.append('file', file);
  await api.post(`/orders/${orderId}/deliverables`, form);
}

export async function sealDelivery(orderId: string): Promise<void> {
  await api.post(`/orders/${orderId}/deliverables/seal`);
}

export function getDeliverableDownloadUrl(orderId: string, fileId: string): string {
  return `/api/v1/orders/${orderId}/deliverables/${fileId}`;
}
