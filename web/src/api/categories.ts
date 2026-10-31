import { api } from '../api';

export interface Category {
  id: string;
  slug: string;
  name: string;
  description: string;
  sort_order: number;
  is_active: boolean;
  parent_id?: string;
  created_at_unix: number;
  updated_at_unix: number;
}

export async function listTop(): Promise<Category[]> {
  const { data } = await api.get<{ data: Category[] }>('/categories', {
    params: { parent: 'top', only_active: true },
  });
  return data.data;
}

export async function listAll(): Promise<Category[]> {
  const { data } = await api.get<{ data: Category[] }>('/categories', {
    params: { only_active: true },
  });
  return data.data;
}

export async function listChildren(parentSlug: string): Promise<Category[]> {
  const { data } = await api.get<{ data: Category[] }>(
    `/categories/slug/${parentSlug}/children`,
    { params: { only_active: true } }
  );
  return data.data;
}

export async function getBySlug(slug: string): Promise<Category> {
  const { data } = await api.get<{ data: Category }>(`/categories/slug/${slug}`);
  return data.data;
}
