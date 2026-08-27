import { getUpdate } from '$lib/sdk';

class VersionManager {
	current = $state('');
	latest = $state('');
	updateAvailable = $state(false);
	private loaded = false;

	async load() {
		if (this.loaded) return;
		this.loaded = true;
		try {
			const status = await getUpdate();
			this.current = status.current_version ?? '';
			this.latest = status.latest_version ?? '';
			this.updateAvailable = status.update_available ?? false;
		} catch {
			this.loaded = false;
		}
	}

	set(current: string, latest: string, available: boolean) {
		this.current = current;
		this.latest = latest;
		this.updateAvailable = available;
	}
}

export const version = new VersionManager();
