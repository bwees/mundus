<script lang="ts">
	import { toggleMode } from 'mode-watcher';
	import { Button } from '$lib/components/ui/button';
	import { page } from '$app/state';
	import { version } from '$lib/managers/version.svelte';
	import { logoutAndRedirect } from '$lib/utils/auth';
	import Gauge from '@lucide/svelte/icons/gauge';
	import Map from '@lucide/svelte/icons/map';
	import Settings from '@lucide/svelte/icons/settings';
	import SunMoon from '@lucide/svelte/icons/sun-moon';
	import LogOut from '@lucide/svelte/icons/log-out';

	let { children } = $props();

	const nav = [
		{ href: '/', label: 'Dashboard', icon: Gauge },
		{ href: '/map', label: 'Map', icon: Map },
		{ href: '/system', label: 'System', icon: Settings }
	];

	function active(href: string) {
		return href === '/' ? page.url.pathname === '/' : page.url.pathname.startsWith(href);
	}

	version.load();
</script>

<div class="bg-background text-foreground flex min-h-screen flex-col">
	<nav class="bg-background/80 sticky top-0 z-10 border-b backdrop-blur">
		<div class="mx-auto flex max-w-4xl items-center gap-1 p-2 sm:px-6">
			<span class="mr-2 font-semibold">Mundus</span>
			{#each nav as n (n.href)}
				<a
					href={n.href}
					class="flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium transition-colors {active(
						n.href
					)
						? 'bg-secondary text-secondary-foreground'
						: 'text-muted-foreground hover:text-foreground'}"
				>
					<n.icon class="size-4" />
					<span class="hidden sm:inline">{n.label}</span>
				</a>
			{/each}
			<div class="flex-1"></div>
			<Button variant="ghost" size="icon" onclick={toggleMode} aria-label="Toggle theme">
				<SunMoon class="size-5" />
			</Button>
			<Button variant="ghost" size="icon" onclick={logoutAndRedirect} aria-label="Sign out">
				<LogOut class="size-5" />
			</Button>
		</div>
	</nav>

	<div class="flex-1">
		{@render children()}
	</div>

	<footer class="text-muted-foreground mx-auto w-full max-w-4xl px-4 py-6 text-xs sm:px-6">
		mundus {version.current}{#if version.updateAvailable}
			· <a href="/system" class="underline">{version.latest} available</a>
		{/if}
	</footer>
</div>
