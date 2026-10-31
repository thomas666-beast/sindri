import { api } from '../api';

export interface Review {
  id: string;
  order_id: string;
  buyer_id: string;
  buyer_username?: string;
  seller_id: string;
  product_id?: string;
  rating: number;
  comment: string;
  created_at_unix: number;
}

export interface Summary {
  avg_rating: number;
  review_count: number;
}

export interface ReviewPage {
  reviews: Review[];
  total: number;
  limit: number;
  offset: number;
}

export async function create(input: {
  order_id: string;
  rating: number;
  comment: string;
}): Promise<Review> {
  const { data } = await api.post<{ data: Review }>('/reviews', input);
  return data.data;
}

export async function listBySeller(
  sellerId: string,
  limit = 20,
  offset = 0
): Promise<ReviewPage> {
  const { data } = await api.get<{ data: ReviewPage }>(`/reviews/seller/${sellerId}`, {
    params: { limit, offset },
  });
  return data.data;
}

export async function listByProduct(
  productId: string,
  limit = 20,
  offset = 0
): Promise<ReviewPage> {
  const { data } = await api.get<{ data: ReviewPage }>(`/reviews/product/${productId}`, {
    params: { limit, offset },
  });
  return data.data;
}

export async function sellerSummary(sellerId: string): Promise<Summary> {
  const { data } = await api.get<{ data: Summary }>(
    `/reviews/seller/${sellerId}/summary`
  );
  return data.data;
}

export async function productSummary(productId: string): Promise<Summary> {
  const { data } = await api.get<{ data: Summary }>(
    `/reviews/product/${productId}/summary`
  );
  return data.data;
}
