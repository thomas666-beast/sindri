import { api } from '../api';

export interface Conversation {
  id: string;
  kind: 'dm' | 'group';
  title?: string;
  members: Member[];
  member_count: number;
  last_message_at_unix?: number;
  last_message_text?: string;
  unread_count: number;
  created_at_unix: number;
}

export interface Member {
  id: string;
  username: string;
  last_read_at_unix?: number;
}

export interface Attachment {
  id: string;
  filename: string;
  content_type: string;
  size_bytes: number;
}

export interface Message {
  id: string;
  conversation_id: string;
  author_id: string;
  body: string;
  created_at_unix: number;
  attachment?: Attachment;
}

export interface MessagePage {
  messages: Message[];
  next_cursor?: string;
  has_more: boolean;
}

export async function listConversations(): Promise<Conversation[]> {
  const { data } = await api.get<{ data: Conversation[] }>('/chat/conversations');
  return data.data;
}

export async function startDMByUsername(username: string): Promise<Conversation> {
  const { data } = await api.post<{ data: Conversation }>('/chat/conversations/dm', {
    username,
  });
  return data.data;
}

export async function createGroupByUsernames(
  title: string,
  usernames: string[]
): Promise<Conversation> {
  const { data } = await api.post<{ data: Conversation }>(
    '/chat/conversations/group',
    { title, usernames }
  );
  return data.data;
}

export async function listMessages(
  convId: string,
  before = '',
  limit = 50
): Promise<MessagePage> {
  const { data } = await api.get<{ data: MessagePage }>(
    `/chat/conversations/${convId}/messages`,
    { params: { before, limit } }
  );
  return data.data;
}

export async function markRead(conversationId: string): Promise<void> {
  await api.post(`/chat/conversations/${conversationId}/read`);
}

export async function uploadAttachment(convId: string, file: File): Promise<Message> {
  const formData = new FormData();
  formData.append('file', file);
  const { data } = await api.post<{ data: Message }>(`/chat/conversations/${convId}/upload`, formData, {
    headers: { 'Content-Type': 'multipart/form-data' }
  });
  return data.data;
}

export function getAttachmentDownloadUrl(id: string): string {
  return `/api/v1/chat/attachments/${id}`;
}
