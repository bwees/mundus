<script lang="ts">
	import '../app.css';
	import { ModeWatcher } from 'mode-watcher';
	import { Toaster } from '$lib/components/ui/sonner';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { auth } from '$lib/managers/auth.svelte';
	import { setUnauthorizedHandler } from '$lib/sdk';

	let { children } = $props();

	// Any request rejected mid-session lands here, not just the one on page load.
	setUnauthorizedHandler(() => {
		auth.logout();
		if (page.url.pathname !== '/login' && page.url.pathname !== '/setup') {
			goto('/login');
		}
	});
</script>

<ModeWatcher />
<Toaster richColors position="top-center" />

{@render children()}
