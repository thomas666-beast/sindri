import { api } from '../api';
import type { Order } from './orders';

export interface AdminUser {
  id: string;
  email: string;
  username: string;
  preferred_currency: string;
  is_admin: boolean;
  is_active: boolean;
  kyc_status: 'not_required' | 'pending' | 'verified' | 'rejected';
  kyc_override: boolean;
  disabled_at_unix?: number;
  disable_reason?: string;
  created_at_unix: number;
}

export interface Warning {
  id: string;
  user_id: string;
  issued_by: string;
  reason: string;
  severity: 'info' | 'warning' | 'severe';
  acknowledged_at_unix?: number;
  created_at_unix: number;
}

export interface Ticket {
  id: string;
  user_id: string;
  user_username: string;
  subject: string;
  status: 'open' | 'answered' | 'closed';
  priority: 'low' | 'normal' | 'high';
  created_at_unix: number;
  updated_at_unix: number;
}

export interface TicketMessage {
  id: string;
  ticket_id: string;
  author_id: string;
  is_staff: boolean;
  body: string;
  created_at_unix: number;
}

export interface AdminStats {
  users_active: number;
  users_disabled: number;
  orders_held: number;
  orders_disputed: number;
  pending_withdrawals: number;
  open_tickets: number;
  commission_by_currency: Record<string, number>;
}

export async function stats(): Promise<AdminStats> {
  const { data } = await api.get<{ data: AdminStats }>('/admin/stats');
  return data.data;
}

// users
export async function listUsers(
  search = '',
  status: '' | 'active' | 'disabled' = '',
  limit = 50,
  offset = 0
): Promise<{ users: AdminUser[]; total: number }> {
  const { data } = await api.get<{ data: AdminUser[]; total: number }>('/admin/users', {
    params: { search, status, limit, offset },
  });
  return { users: data.data, total: data.total };
}

export async function getUser(id: string): Promise<AdminUser> {
  const { data } = await api.get<{ data: AdminUser }>(`/admin/users/${id}`);
  return data.data;
}

export async function disableUser(id: string, reason: string): Promise<AdminUser> {
  const { data } = await api.post<{ data: AdminUser }>(`/admin/users/${id}/disable`, {
    reason,
  });
  return data.data;
}

export async function enableUser(id: string): Promise<AdminUser> {
  const { data } = await api.post<{ data: AdminUser }>(`/admin/users/${id}/enable`);
  return data.data;
}

export async function setKYC(
  id: string,
  status: 'not_required' | 'pending' | 'verified' | 'rejected'
): Promise<AdminUser> {
  const { data } = await api.post<{ data: AdminUser }>(`/admin/users/${id}/kyc`, {
    status,
  });
  return data.data;
}

export async function setAdmin(id: string, isAdmin: boolean): Promise<AdminUser> {
  const { data } = await api.post<{ data: AdminUser }>(`/admin/users/${id}/admin`, {
    is_admin: isAdmin,
  });
  return data.data;
}

// warnings
export async function listWarnings(userId: string): Promise<Warning[]> {
  const { data } = await api.get<{ data: Warning[] }>(`/admin/users/${userId}/warnings`);
  return data.data;
}

export async function createWarning(
  userId: string,
  reason: string,
  severity: 'info' | 'warning' | 'severe'
): Promise<Warning> {
  const { data } = await api.post<{ data: Warning }>(`/admin/users/${userId}/warnings`, {
    user_id: userId,
    reason,
    severity,
  });
  return data.data;
}

// tickets
export async function listTickets(
  status: '' | 'open' | 'answered' | 'closed' = '',
  limit = 50,
  offset = 0
): Promise<Ticket[]> {
  const { data } = await api.get<{ data: Ticket[] }>('/admin/tickets', {
    params: { status, limit, offset },
  });
  return data.data;
}

export async function getTicket(id: string): Promise<Ticket> {
  const { data } = await api.get<{ data: Ticket }>(`/tickets/${id}`);
  return data.data;
}

export async function listTicketMessages(id: string): Promise<TicketMessage[]> {
  const { data } = await api.get<{ data: TicketMessage[] }>(`/tickets/${id}/messages`);
  return data.data;
}

export async function addTicketMessage(
  id: string,
  body: string
): Promise<TicketMessage> {
  const { data } = await api.post<{ data: TicketMessage }>(`/tickets/${id}/messages`, {
    body,
  });
  return data.data;
}

export async function setTicketStatus(
  id: string,
  status: 'open' | 'answered' | 'closed'
): Promise<Ticket> {
  const { data } = await api.post<{ data: Ticket }>(`/admin/tickets/${id}/status`, {
    status,
  });
  return data.data;
}

// ---- WITHDRAWALS ----

export interface Withdrawal {
  id: string;
  user_id: string;
  username: string;
  currency_code: string;
  amount: number;
  status: 'pending' | 'approved' | 'rejected';
  rejection_reason?: string;
  reviewed_by?: string;
  reviewed_by_username?: string;
  reviewed_at_unix?: number;
  created_at_unix: number;
}

// Lists withdrawals. Use status='' to get all, or 'pending' to get the queue.
export async function listWithdrawals(
  status: '' | 'pending' | 'approved' | 'rejected' = '',
  id = '', // <-- New ID param
  limit = 50,
  offset = 0
): Promise<Withdrawal[]> {
  const { data } = await api.get<{ data: Withdrawal[] }>('/admin/withdrawals', {
    params: { status, id, limit, offset }, // <-- Pass it
  });
  return data.data;
}

export async function approveWithdrawal(id: string): Promise<Withdrawal> {
  const { data } = await api.post<{ data: Withdrawal }>(`/admin/withdrawals/${id}/approve`);
  return data.data;
}

export async function rejectWithdrawal(id: string, reason: string): Promise<Withdrawal> {
  const { data } = await api.post<{ data: Withdrawal }>(`/admin/withdrawals/${id}/reject`, {
    reason,
  });
  return data.data;
}

// ---- DISPUTED ORDERS ----

export interface DisputedOrdersPage {
  orders: Order[];
  total: number;
}

export async function listDisputedOrders(
  limit = 100,
  offset = 0,
  id = ''
): Promise<Order[]> {
  const { data } = await api.get<{ data: Order[] }>('/admin/orders/disputed', {
    params: { limit, offset, id },
  });
  return data.data;
}

export async function releaseOrder(id: string): Promise<Order> {
  const { data } = await api.post<{ data: Order }>(
    `/admin/orders/${id}/release`
  );
  return data.data;
}

export async function refundOrder(id: string): Promise<Order> {
  const { data } = await api.post<{ data: Order }>(
    `/admin/orders/${id}/refund`
  );
  return data.data;
}

export async function getOrder(id: string): Promise<Order> {
  const { data } = await api.get<{ data: Order }>(`/admin/orders/${id}`);
  return data.data;
}

export interface AdminTransaction {
  id: string;
  user_id: string;
  username: string;
  currency_code: string;
  type: string;
  direction: 'credit' | 'debit';
  amount: number;
  balance_after: number;
  order_id?: string;
  status: string;
  metadata: string;
  created_at_unix: number;
}

export async function listTransactions(
  type = '',
  id = '',
  user = '',
  limit = 50,
  offset = 0
): Promise<AdminTransaction[]> {
  const { data } = await api.get<{ data: AdminTransaction[] }>('/admin/transactions', {
    params: { type, id, user, limit, offset },
  });
  return data.data;
}

export async function getSettings(): Promise<Record<string, string>> {
  const { data } = await api.get<{ data: Record<string, string> }>('/admin/settings');
  return data.data;
}

export async function updateSetting(key: string, value: string): Promise<void> {
  await api.put(`/admin/settings/${key}`, { value: String(value) });
}
