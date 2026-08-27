import * as client from './client';

// Same-origin: the Go server serves both the app and /api.
client.defaults.baseUrl = '';

const TOKEN_KEY = 'mundus.token';

let token: string | null = null;

function applyToken() {
	client.defaults.headers = token ? { Authorization: `Bearer ${token}` } : {};
}

export function loadToken(): string | null {
	if (token === null && typeof localStorage !== 'undefined') {
		token = localStorage.getItem(TOKEN_KEY);
		applyToken();
	}
	return token;
}

export function setToken(next: string | null) {
	token = next;
	if (typeof localStorage !== 'undefined') {
		if (next) localStorage.setItem(TOKEN_KEY, next);
		else localStorage.removeItem(TOKEN_KEY);
	}
	applyToken();
}

export function isUnauthorized(error: unknown): boolean {
	return (error as { status?: number })?.status === 401;
}

export function apiErrorMessage(error: unknown, fallback = 'Something went wrong. Please try again.'): string {
	const data = (error as { data?: { detail?: string; title?: string; error?: string } })?.data;
	return data?.detail ?? data?.title ?? data?.error ?? fallback;
}

export * from './client';
