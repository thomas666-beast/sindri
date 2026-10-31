import { api } from '../api';

export interface Withdrawal {
  id: string;
  user_id: string;
  currency_code: string;
  amount: number;
  status: 'pending' | 'approved' | 'rejected';
  rejection_reason?: string;
  reviewed_by?: string;
  reviewed_at_unix?: number;
  created_at_unix: number;
}

export async function request(input: {
  currency: string;
  amount: number;
}): Promise<Withdrawal> {
  const { data } = await api.post<{ data: Withdrawal }>('/withdrawals', input);
  return data.data;
}

export async function listMine(
  limit = 30,
  offset = 0
): Promise<Withdrawal[]> {
  const { data } = await api.get<{ data: Withdrawal[] }>('/withdrawals', {
    params: { limit, offset },
  });
  return data.data;
}

export async function adminListPending(
  limit = 100,
  offset = 0
): Promise<Withdrawal[]> {
  const { data } = await api.get<{ data: Withdrawal[] }>('/admin/withdrawals', {
    params: { limit, offset },
  });
  return data.data;
}

export async function adminApprove(id: string): Promise<Withdrawal> {
  const { data } = await api.post<{ data: Withdrawal }>(
    `/admin/withdrawals/${id}/approve`
  );
  return data.data;
}

export async function adminReject(id: string, reason: string): Promise<Withdrawal> {
  const { data } = await api.post<{ data: Withdrawal }>(
    `/admin/withdrawals/${id}/reject`,
    { reason }
  );
  return data.data;
}
