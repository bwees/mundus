import { getAuthStatus, loadToken, login, setToken, setupAuth } from '$lib/sdk';

class AuthManager {
	authenticated = $state(false);
	private loaded = false;

	// A token in storage is only a claim; the server decides. The status call is
	// unauthenticated, so a stale token surfaces as a failed probe, not a hang.
	async load(): Promise<boolean> {
		if (!this.loaded) {
			this.authenticated = loadToken() !== null;
			this.loaded = true;
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
		this.loaded = true;
	}

	async setup(password: string) {
		const { token } = await setupAuth({ password });
		setToken(token);
		this.authenticated = true;
		this.loaded = true;
	}

	logout() {
		setToken(null);
		this.authenticated = false;
		this.loaded = true;
	}
}

export const auth = new AuthManager();
