<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/managers/auth.svelte';
	import { apiErrorMessage } from '$lib/sdk';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import ShieldCheck from '@lucide/svelte/icons/shield-check';

	const MIN_LENGTH = 8;

	let password = $state('');
	let confirm = $state('');
	let loading = $state(false);
	let error = $state('');

	let tooShort = $derived(password.length > 0 && password.length < MIN_LENGTH);
	let mismatch = $derived(confirm.length > 0 && password !== confirm);
	let valid = $derived(password.length >= MIN_LENGTH && password === confirm);

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		loading = true;
		try {
			await auth.setup(password);
			await goto('/');
		} catch (err) {
			error = apiErrorMessage(err, 'Could not set the password.');
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head><title>Set up · Mundus</title></svelte:head>

<div class="flex min-h-screen items-center justify-center p-4">
	<Card.Root class="w-full sm:max-w-md">
		<Card.Header>
			<Card.Title class="flex items-center gap-2">
				<ShieldCheck class="size-5" /> Secure this robot
			</Card.Title>
			<Card.Description>
				Anyone on this network can reach mundus until an admin password is set. Choose one now —
				you will need it to sign in from any device.
			</Card.Description>
		</Card.Header>
		<Card.Content>
			<form class="grid gap-4" onsubmit={handleSubmit}>
				<div class="grid gap-2">
					<Label for="password">Admin password</Label>
					<Input
						id="password"
						type="password"
						bind:value={password}
						autocomplete="new-password"
						required
					/>
					<p class="text-muted-foreground text-xs">
						{#if tooShort}
							<span class="text-destructive">At least {MIN_LENGTH} characters.</span>
						{:else}
							At least {MIN_LENGTH} characters.
						{/if}
					</p>
				</div>
				<div class="grid gap-2">
					<Label for="confirm">Confirm password</Label>
					<Input
						id="confirm"
						type="password"
						bind:value={confirm}
						autocomplete="new-password"
						required
					/>
					{#if mismatch}<p class="text-destructive text-xs">Passwords do not match.</p>{/if}
				</div>
				{#if error}<p class="text-destructive text-sm">{error}</p>{/if}
				<Button type="submit" class="w-full" disabled={loading || !valid}>
					{loading ? 'Saving…' : 'Set password and continue'}
				</Button>
			</form>
		</Card.Content>
	</Card.Root>
</div>
