import { api } from '../api';

export interface Product {
  id: string;
  seller_id: string;
  title: string;
  description: string;
  price: number;
  seller_username?: string;
  currency_code: string;
  cover_image?: string;
  category_id?: string;
  created_at_unix: number;
}

export interface CreateProductInput {
  title: string;
  description: string;
  price: number;
  currency_code: string;
  category_id?: string;
}

export async function list(limit = 50, offset = 0): Promise<Product[]> {
  const { data } = await api.get<{ data: Product[] }>('/products', {
    params: { limit, offset },
  });
  return data.data;
}

export async function get(id: string): Promise<Product> {
  const { data } = await api.get<{ data: Product }>(`/products/${id}`);
  return data.data;
}

export async function create(input: CreateProductInput): Promise<Product> {
  const { data } = await api.post<{ data: Product }>('/products', input);
  return data.data;
}

export async function deactivate(id: string): Promise<void> {
  await api.delete(`/products/${id}`);
}

export interface SearchParams {
  q?: string;
  category?: string;
  seller?: string;
  id?: string;
  limit?: number;
  offset?: number;
}

export interface SearchResult {
  results: Product[];
  total: number;
  limit: number;
  offset: number;
}

export async function search(params: SearchParams): Promise<SearchResult> {
  const { data } = await api.get<{ data: SearchResult }>('/products/search', {
    params: {
      q: params.q ?? '',
      category: params.category ?? '',
      seller: params.seller ?? '',
      id: params.id ?? '',
      limit: params.limit ?? 30,
      offset: params.offset ?? 0,
    },
  });
  return data.data;
}

export interface UpdateProductInput {
  title: string;
  description: string;
  price: number;
  category_id?: string;
}

export async function update(
  id: string,
  input: UpdateProductInput,
): Promise<Product> {
  const { data } = await api.put<{ data: Product }>(`/products/${id}`, input);
  return data.data;
}

export interface ProductImage {
  id: string;
  url: string;
}

export async function uploadImage(productId: string, file: File): Promise<string> {
  const form = new FormData();
  form.append('image', file);
  const { data } = await api.post<{ url: string }>(`/products/${productId}/images`, form, {
    headers: { 'Content-Type': 'multipart/form-data' }
  });
  return data.url;
}

export async function getImages(productId: string): Promise<string[]> {
  const { data } = await api.get<{ data: string[] }>(`/products/${productId}/images`);
  return data.data;
}
