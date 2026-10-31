import axios from 'axios';

export const api = axios.create({
  baseURL: '/api/v1',
  withCredentials: true,
});

export interface User {
  id: string;
  email: string;
  username: string;
  preferred_currency: string;
  is_admin: boolean;
  kyc_status: string;
}

export interface AuthResponse {
  user: User;
  token?: string;
}

export async function login(input: {
  email: string;
  password: string;
}): Promise<AuthResponse> {
  const { data } = await api.post<AuthResponse>('/auth/login', input);
  return data;
}

export async function register(input: {
  email: string;
  username: string;
  password: string;
}): Promise<AuthResponse> {
  const { data } = await api.post<AuthResponse>('/auth/register', input);
  return data;
}

export async function logout(): Promise<void> {
  await api.post('/auth/logout');
}

export async function me(): Promise<User> {
  const { data } = await api.get<{ user: User }>('/me');
  return data.user;
}

export function getErrorMessage(err: any): string {
  if (!err) return 'An unknown error occurred';
  
  // 1. Check for backend validation errors (HTTP 422)
  const body = err.response?.data;
  if (body?.errors) {
    // If it's an object of field errors, join them
    if (typeof body.errors === 'object') {
      return Object.values(body.errors).join(', ');
    }
    // If it's a string
    return String(body.errors);
  }

  // 2. Check for standard backend error (HTTP 400, 404, 500)
  if (body?.error) {
    return body.error;
  }

  // 3. Axios timeout or network error
  if (err.message === 'Network Error') {
    return 'Network error — check your connection.';
  }

  // 4. Fallback to the Axios message or stringified error
  return err.message || 'Request failed';
}
