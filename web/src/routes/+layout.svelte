<script lang="ts">
	import '../app.css';
	import { ModeWatcher } from 'mode-watcher';
	import { Toaster } from '$lib/components/ui/sonner';
	import { loadToken } from '$lib/sdk';

	let { children } = $props();

	// Restore the bearer token before any load function fires a request.
	loadToken();
</script>

<ModeWatcher />
<Toaster richColors position="top-center" />

{@render children()}
