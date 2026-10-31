import { api } from '../api';

export interface DashboardStats {
  total_earned: number;
  active_orders: number;
  pending_withdrawals: number;
  active_products: number;
}

export async function getStats(): Promise<DashboardStats> {
  const { data } = await api.get<{ data: DashboardStats }>('/dashboard/stats');
  return data.data;
}
