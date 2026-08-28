import { getAuthStatus, getSession, login, logout, setupAuth } from '$lib/sdk';

class AuthManager {
	authenticated = $state(false);
	private checked = false;

	// The session cookie is HttpOnly, so the only way to know whether we are
	// signed in is to ask. The server signs tokens with a key held in memory,
	// so a cookie that survived a restart is already worthless.
	async load(): Promise<boolean> {
		if (this.checked) return this.authenticated;
		this.checked = true;
		try {
			await getSession();
			this.authenticated = true;
		} catch {
			this.authenticated = false;
		}
		return this.authenticated;
	}

	async setupRequired(): Promise<boolean> {
		try {
			return (await getAuthStatus()).setup_required ?? false;
		} catch {
			return false;
		}
	}

	async login(password: string) {
		await login({ password });
		this.authenticated = true;
		this.checked = true;
	}

	async setup(password: string) {
		await setupAuth({ password });
		this.authenticated = true;
		this.checked = true;
	}

	// Only the server can clear an HttpOnly cookie.
	async logout() {
		try {
			await logout();
		} finally {
			this.authenticated = false;
			this.checked = true;
		}
	}
}

export const auth = new AuthManager();
