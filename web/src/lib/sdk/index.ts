import * as client from './client';

// Same-origin: the Go server serves both the app and /api.
client.defaults.baseUrl = '';

const TOKEN_KEY = 'mundus.token';

// Routes whose caller deals with its own 401: a wrong password on login/setup,
// and the session probe, which reports the answer rather than reacting to it.
// Firing the global handler for those would race a second navigation against
// the one the caller is already doing.
const SELF_HANDLED = [
	'/api/auth/login',
	'/api/auth/setup',
	'/api/auth/status',
	'/api/auth/session'
];

let token: string | null = null;
let onUnauthorized: (() => void) | null = null;

function applyToken() {
	client.defaults.headers = token ? { Authorization: `Bearer ${token}` } : {};
}

function urlOf(input: RequestInfo | URL): string {
	if (typeof input === 'string') return input;
	if (input instanceof URL) return input.pathname;
	return input.url;
}

// Every 401 outside the credential routes means the stored token is no longer
// good, which happens on every server restart because the signing key only
// lives in memory. Drop it here so no caller has to remember to.
client.defaults.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
	const response = await fetch(input, init);
	if (response.status === 401 && !SELF_HANDLED.some((r) => urlOf(input).includes(r))) {
		setToken(null);
		onUnauthorized?.();
	}
	return response;
};

export function setUnauthorizedHandler(fn: () => void) {
	onUnauthorized = fn;
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

export function apiErrorMessage(error: unknown, fallback = 'Something went wrong. Please try again.'): string {
	const data = (error as { data?: { detail?: string; title?: string; error?: string } })?.data;
	return data?.detail ?? data?.title ?? data?.error ?? fallback;
}

export * from './client';
