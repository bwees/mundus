import * as client from './client';

// Same-origin: the Go server serves both the app and /api.
client.defaults.baseUrl = '';

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

let onUnauthorized: (() => void) | null = null;

function urlOf(input: RequestInfo | URL): string {
	if (typeof input === 'string') return input;
	if (input instanceof URL) return input.pathname;
	return input.url;
}

// The session rides in an HttpOnly cookie, so there is no token to attach here
// and nothing for a script to steal. `credentials` is same-origin by default in
// browsers, but it is set explicitly so the intent survives a refactor.
client.defaults.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
	const response = await fetch(input, { ...init, credentials: 'same-origin' });
	if (response.status === 401 && !SELF_HANDLED.some((r) => urlOf(input).includes(r))) {
		onUnauthorized?.();
	}
	return response;
};

export function setUnauthorizedHandler(fn: () => void) {
	onUnauthorized = fn;
}

export function apiErrorMessage(error: unknown, fallback = 'Something went wrong. Please try again.'): string {
	const data = (error as { data?: { detail?: string; title?: string; error?: string } })?.data;
	return data?.detail ?? data?.title ?? data?.error ?? fallback;
}

export * from './client';
