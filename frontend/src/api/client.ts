
import type { ApiEnvelope, UserSession } from '../types/domain';

const TOKEN_KEY = 'domain-control-session';

// ApiError keeps the backend error code and any structured 阻断项 so pages can
// list exactly why a release gate rejected the transition.
export class ApiError extends Error {
  code: string;
  blockers: string[];
  constructor(message: string, code = '', blockers: string[] = []) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
    this.blockers = blockers;
  }
}

export function getToken(): string {
	return readSession()?.token || '';
}
export function readSession(): UserSession | null {
	try {
		const session = JSON.parse(localStorage.getItem(TOKEN_KEY) || 'null') as UserSession | null;
		return session?.token && session.username && session.role ? session : null;
	} catch { return null; }
}
export function saveSession(session: UserSession): void {
	localStorage.setItem(TOKEN_KEY, JSON.stringify(session));
	window.dispatchEvent(new Event('auth-session-changed'));
}
export function clearSession(): void {
	localStorage.removeItem(TOKEN_KEY);
	window.dispatchEvent(new Event('auth-session-changed'));
}

export async function request<T>(path: string, init: RequestInit = {}): Promise<ApiEnvelope<T>> {
  const headers = new Headers(init.headers);
  headers.set('Accept', 'application/json');
  if (init.body) headers.set('Content-Type', 'application/json');
  const token = getToken();
  if (token) headers.set('Authorization', `Bearer ${token}`);
	const response = await fetch(`/api${path}`, { ...init, headers });
	if (response.status === 204) return { data: undefined as T };
	const payload = await response.json().catch(() => ({ error: 'invalid_response', message: '服务返回了无法解析的响应' }));
	if (response.status === 401 && path !== '/auth/login') clearSession();
	if (!response.ok) {
		const blockers = Array.isArray(payload?.meta?.blockers) ? payload.meta.blockers.filter((item: unknown) => typeof item === 'string') : [];
		throw new ApiError(payload.message || payload.error || `HTTP ${response.status}`, payload.error || '', blockers);
	}
  return payload as ApiEnvelope<T>;
}
