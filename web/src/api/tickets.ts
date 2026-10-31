import { api } from '../api';

export interface Ticket {
  id: string;
  user_id: string;
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

export async function create(input: {
  subject: string;
  priority: 'low' | 'normal' | 'high';
}): Promise<Ticket> {
  const { data } = await api.post<{ data: Ticket }>('/tickets', input);
  return data.data;
}

export async function listMine(
  limit = 30,
  offset = 0
): Promise<Ticket[]> {
  const { data } = await api.get<{ data: Ticket[] }>('/tickets', {
    params: { limit, offset },
  });
  return data.data;
}

export async function get(id: string): Promise<Ticket> {
  const { data } = await api.get<{ data: Ticket }>(`/tickets/${id}`);
  return data.data;
}

export async function listMessages(id: string): Promise<TicketMessage[]> {
  const { data } = await api.get<{ data: TicketMessage[] }>(
    `/tickets/${id}/messages`
  );
  return data.data;
}

export async function addMessage(
  id: string,
  body: string
): Promise<TicketMessage> {
  const { data } = await api.post<{ data: TicketMessage }>(
    `/tickets/${id}/messages`,
    { body }
  );
  return data.data;
}

export interface AdminTicket extends Ticket {
  user_username?: string;
}

export async function adminListTickets(
  status: '' | 'open' | 'answered' | 'closed' = '',
  limit = 100,
  offset = 0
): Promise<AdminTicket[]> {
  const { data } = await api.get<{ data: AdminTicket[] }>('/admin/tickets', {
    params: { status, limit, offset },
  });
  return data.data;
}

export async function adminSetStatus(
  id: string,
  status: 'open' | 'answered' | 'closed'
): Promise<Ticket> {
  const { data } = await api.post<{ data: Ticket }>(`/admin/tickets/${id}/status`, {
    status,
  });
  return data.data;
}
