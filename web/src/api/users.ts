import { api } from '../api';

export interface UserHit {
  id: string;
  username: string;
  display_name: string;
  bio: string;
  skills: string[];
  interests: string[];
  avatar_url?: string;
  preferred_currency: string;
  member_since_unix: number;
}

export interface SearchPage {
  results: UserHit[];
  total: number;
  limit: number;
  offset: number;
}

export async function search(
  q = '',
  limit = 30,
  offset = 0
): Promise<SearchPage> {
  const { data } = await api.get<{ data: SearchPage }>('/users/search', {
    params: { q, limit, offset },
  });
  return data.data;
}
