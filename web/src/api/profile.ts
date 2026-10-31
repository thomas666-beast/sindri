import { api } from '../api';

export interface Profile {
  user_id: string;
  username: string;
  display_name: string;
  bio: string;
  skills: string[];
  interests: string[];
  avatar_url?: string;
  member_since_unix: number;
  has_custom_profile: boolean;
}

export interface UpdateInput {
  display_name: string;
  bio: string;
  skills: string[];
  interests: string[];
  avatar_url: string;
}

export async function getMine(): Promise<Profile> {
  const { data } = await api.get<{ data: Profile }>('/me/profile');
  return data.data;
}

export async function update(input: UpdateInput): Promise<Profile> {
  const { data } = await api.put<{ data: Profile }>('/me/profile', input);
  return data.data;
}

export async function getPublic(username: string): Promise<Profile> {
  const { data } = await api.get<{ data: Profile }>(
    `/users/${encodeURIComponent(username)}/profile`
  );
  return data.data;
}

export interface GeekStats {
  completed_orders: number;
  active_products: number;
  avg_rating: number;
  review_count: number;
}

export async function getStats(username: string): Promise<GeekStats> {
  const { data } = await api.get<{ data: GeekStats }>(`/users/${username}/stats`);
  return data.data;
}
