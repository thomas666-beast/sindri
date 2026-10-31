import { api } from '../api';

export interface Notification {
  id: string;
  user_id: string;
  kind: string;
  title: string;
  body: string;
  // metadata is a JSON object from the backend
  metadata: { 
    order_id?: string; 
    amount?: number; 
    currency?: string; 
    buyer_username?: string;
    seller_username?: string;
    reason?: string;
    title?: string;
  };
  read_at_unix?: number;
  created_at_unix: number;
}

export interface NotificationPage {
  notifications: Notification[];
  limit: number;
  offset: number;
}

export async function list(
  onlyUnread = false,
  limit = 30,
  offset = 0
): Promise<NotificationPage> {
  const { data } = await api.get<{ data: NotificationPage }>('/notifications', {
    params: { only_unread: onlyUnread, limit, offset },
  });
  return data.data;
}

export async function unreadCount(): Promise<number> {
  const { data } = await api.get<{ data: { count: number } }>(
    '/notifications/unread-count'
  );
  return data.data.count;
}

export async function markRead(id: string): Promise<void> {
  await api.post(`/notifications/${id}/read`);
}

export async function markAllRead(): Promise<number> {
  const { data } = await api.post<{ data: { updated: number } }>(
    '/notifications/read-all'
  );
  return data.data.updated;
}
