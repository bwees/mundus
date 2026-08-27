<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/managers/auth.svelte';
	import { apiErrorMessage } from '$lib/sdk';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import Lock from '@lucide/svelte/icons/lock';

	let password = $state('');
	let loading = $state(false);
	let error = $state('');

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		loading = true;
		try {
			await auth.login(password);
			await goto('/');
		} catch (err) {
			error = apiErrorMessage(err, 'Incorrect password.');
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head><title>Sign in · Mundus</title></svelte:head>

<div class="flex min-h-screen items-center justify-center p-4">
	<Card.Root class="w-full sm:max-w-md">
		<Card.Header>
			<Card.Title class="flex items-center gap-2"><Lock class="size-5" /> Sign in</Card.Title>
			<Card.Description>Enter the admin password for this robot.</Card.Description>
		</Card.Header>
		<Card.Content>
			<form class="grid gap-4" onsubmit={handleSubmit}>
				<div class="grid gap-2">
					<Label for="password">Password</Label>
					<Input
						id="password"
						type="password"
						bind:value={password}
						autocomplete="current-password"
						required
					/>
				</div>
				{#if error}<p class="text-destructive text-sm">{error}</p>{/if}
				<Button type="submit" class="w-full" disabled={loading || !password}>
					{loading ? 'Signing in…' : 'Sign in'}
				</Button>
			</form>
		</Card.Content>
	</Card.Root>
</div>
