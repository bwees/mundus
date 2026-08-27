import { getAuthStatus, getSession, loadToken, login, setToken, setupAuth } from '$lib/sdk';

class AuthManager {
	authenticated = $state(false);
	private checked = false;

	// A token in storage proves nothing: the server signs with a key held only in
	// memory, so every restart invalidates it. Ask the server instead of trusting
	// localStorage, and drop a token it rejects.
	async load(): Promise<boolean> {
		if (this.checked) return this.authenticated;
		this.checked = true;

		if (loadToken() === null) {
			this.authenticated = false;
			return false;
		}
		try {
			await getSession();
			this.authenticated = true;
		} catch {
			setToken(null);
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
		const { token } = await login({ password });
		setToken(token);
		this.authenticated = true;
		this.checked = true;
	}

	async setup(password: string) {
		const { token } = await setupAuth({ password });
		setToken(token);
		this.authenticated = true;
		this.checked = true;
	}

	logout() {
		setToken(null);
		this.authenticated = false;
		this.checked = true;
	}
}

export const auth = new AuthManager();
