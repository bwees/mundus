<script lang="ts">
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import * as api from '$lib/sdk';
	import { version } from '$lib/managers/version.svelte';
	import type { MqttConfigDto, CloudStatus, SshStatus, Status as UpdateStatus } from '$lib/sdk';

	import * as Card from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Badge } from '$lib/components/ui/badge';
	import { Switch } from '$lib/components/ui/switch';
	import { Separator } from '$lib/components/ui/separator';
	import Plug from '@lucide/svelte/icons/plug';
	import Cloud from '@lucide/svelte/icons/cloud';
	import Terminal from '@lucide/svelte/icons/terminal';
	import Download from '@lucide/svelte/icons/download';
	import Trash2 from '@lucide/svelte/icons/trash-2';
	import KeyRound from '@lucide/svelte/icons/key-round';

	let mqtt = $state<MqttConfigDto | null>(null);
	let broker = $state(''), username = $state(''), password = $state(''), baseTopic = $state(''), discoveryPrefix = $state('');
	let savingMqtt = $state(false);

	let cloud = $state<CloudStatus | null>(null);
	let ssh = $state<SshStatus | null>(null);
	let newKey = $state('');
	let busy = $state('');

	let upd = $state<UpdateStatus | null>(null);
	let checking = $state(false);
	let applying = $state(false);

	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let changingPassword = $state(false);

	let passwordValid = $derived(
		currentPassword.length > 0 && newPassword.length >= 8 && newPassword === confirmPassword
	);

	async function changePassword() {
		changingPassword = true;
		try {
			const { token } = await api.changePassword({
				current_password: currentPassword,
				new_password: newPassword
			});
			api.setToken(token);
			currentPassword = newPassword = confirmPassword = '';
			toast.success('Password changed');
		} catch (e) {
			toast.error(api.apiErrorMessage(e, 'Could not change the password'));
		} finally {
			changingPassword = false;
		}
	}

	async function loadMqtt() {
		try {
			mqtt = await api.getMqttConfig();
			broker = mqtt.broker; username = mqtt.username; baseTopic = mqtt.base_topic; discoveryPrefix = mqtt.discovery_prefix;
		} catch (e) { toast.error(`MQTT: ${e}`); }
	}
	async function saveMqtt() {
		savingMqtt = true;
		try {
			mqtt = await api.setMqttConfig({ broker, username, password, base_topic: baseTopic, discovery_prefix: discoveryPrefix });
			password = '';
			toast.success('Saved — reconnecting to broker');
			setTimeout(loadMqtt, 2500);
		} catch (e) { toast.error(`Save failed: ${e}`); }
		finally { savingMqtt = false; }
	}

	async function loadCloud() { try { cloud = await api.getCloud(); } catch (e) { toast.error(`Cloud: ${e}`); } }
	async function toggleCloud(enabled: boolean) {
		busy = 'cloud';
		try { await api.setCloud({ enabled }); toast.success(enabled ? 'Cloud enabled' : 'Cloud disabled (effective on reconnect/reboot)'); await loadCloud(); }
		catch (e) { toast.error(`Cloud: ${e}`); }
		finally { busy = ''; }
	}

	async function loadSsh() { try { ssh = await api.getSsh(); } catch (e) { toast.error(`SSH: ${e}`); } }
	async function toggleSsh(enabled: boolean) {
		busy = 'ssh';
		try { await api.setSsh({ enabled }); toast.success(enabled ? 'SSH enabled on the network' : 'SSH restricted to localhost'); await loadSsh(); }
		catch (e) { toast.error(`SSH: ${e}`); }
		finally { busy = ''; }
	}
	async function addKey() {
		if (!newKey.trim()) return;
		busy = 'addkey';
		try { await api.addSshKey({ key: newKey.trim() }); newKey = ''; toast.success('Key added'); await loadSsh(); }
		catch (e) { toast.error(`Add key: ${e}`); }
		finally { busy = ''; }
	}
	async function removeKey(blob: string) {
		busy = 'rmkey';
		try { await api.deleteSshKey({ key: blob }); toast.success('Key removed'); await loadSsh(); }
		catch (e) { toast.error(`Remove key: ${e}`); }
		finally { busy = ''; }
	}

	async function loadUpdate() { try { upd = await api.getUpdate(); } catch (e) { toast.error(`Update: ${e}`); } }
	async function checkUpdate() {
		checking = true;
		try {
			upd = await api.checkUpdate();
			version.set(upd.current_version ?? '', upd.latest_version ?? '', upd.update_available ?? false);
			toast.success(upd.update_available ? `Update available: ${upd.latest_version}` : 'Up to date');
		}
		catch (e) { toast.error(`Check failed: ${e}`); }
		finally { checking = false; }
	}
	async function applyUpdate() {
		applying = true;
		try {
			await api.applyUpdate();
			toast.success('Updating — the server will restart. Reloading shortly…');
			setTimeout(() => location.reload(), 6000);
		} catch (e) { toast.error(`Update failed: ${e}`); applying = false; }
	}

	onMount(() => { loadMqtt(); loadCloud(); loadSsh(); loadUpdate(); });
</script>

<main>
	<div class="mx-auto max-w-2xl space-y-6 p-4 sm:p-6">
		<h1 class="text-2xl font-bold tracking-tight">System</h1>

		<Card.Root>
			<Card.Header>
				<Card.Title class="flex items-center gap-2">
					<Plug class="size-5" /> MQTT / Home Assistant
					{#if mqtt}<Badge class="ml-auto" variant={mqtt.connected ? 'default' : 'destructive'}>{mqtt.connected ? 'Connected' : 'Disconnected'}</Badge>{/if}
				</Card.Title>
			</Card.Header>
			<Card.Content class="space-y-4">
				<div class="space-y-1.5">
					<Label for="broker">Broker URL</Label>
					<Input id="broker" bind:value={broker} placeholder="tcp://mqtt.example.com:1883" />
				</div>
				<div class="grid gap-4 sm:grid-cols-2">
					<div class="space-y-1.5"><Label for="u">Username</Label><Input id="u" bind:value={username} placeholder="(optional)" autocomplete="off" /></div>
					<div class="space-y-1.5"><Label for="p">Password</Label><Input id="p" type="password" bind:value={password} placeholder={mqtt?.has_password ? '•••••• (unchanged)' : '(optional)'} autocomplete="new-password" /></div>
				</div>
				<div class="grid gap-4 sm:grid-cols-2">
					<div class="space-y-1.5"><Label for="bt">Base Topic</Label><Input id="bt" bind:value={baseTopic} placeholder="mundus" /></div>
					<div class="space-y-1.5"><Label for="dp">Discovery Prefix</Label><Input id="dp" bind:value={discoveryPrefix} placeholder="homeassistant" /></div>
				</div>
			</Card.Content>
			<Card.Footer><Button onclick={saveMqtt} disabled={savingMqtt || !broker}>{savingMqtt ? 'Saving…' : 'Save & Connect'}</Button></Card.Footer>
		</Card.Root>

		<Card.Root>
			<Card.Header>
				<Card.Title class="flex items-center gap-2"><Cloud class="size-5" /> SwitchBot Cloud</Card.Title>
			</Card.Header>
			<Card.Content class="space-y-3">
				{#if cloud}
					<div class="flex items-center justify-between">
						<Label class="font-normal">Cloud connectivity</Label>
						<Switch checked={cloud.enabled} disabled={busy === 'cloud'} onCheckedChange={toggleCloud} />
					</div>
					<div class="flex gap-2 text-xs">
						<Badge variant={cloud.connected ? 'default' : 'secondary'}>{cloud.connected ? 'Connected' : 'Not connected'}</Badge>
						<Badge variant="outline">{cloud.bound ? 'Account-bound' : 'Unbound'}</Badge>
					</div>
				{/if}
			</Card.Content>
		</Card.Root>

		<Card.Root>
			<Card.Header>
				<Card.Title class="flex items-center gap-2"><Terminal class="size-5" /> SSH Access</Card.Title>
			</Card.Header>
			<Card.Content class="space-y-4">
				{#if ssh}
					<div class="flex items-center justify-between">
						<Label class="font-normal">Enable Local SSH Access</Label>
						<Switch checked={ssh.enabled} disabled={busy === 'ssh'} onCheckedChange={toggleSsh} />
					</div>
					<Separator />
					<div class="space-y-2">
						<Label>Authorized keys</Label>
						{#each ssh.keys ?? [] as k (k.key)}
							<div class="flex items-center gap-2 rounded-md border p-2 text-sm">
								<span class="text-muted-foreground shrink-0">{k.type}</span>
								<code class="flex-1 truncate text-xs">{k.comment || k.key.slice(0, 24) + '…'}</code>
								<button type="button" class="text-destructive p-1" title="Remove" onclick={() => removeKey(k.key)}><Trash2 class="size-4" /></button>
							</div>
						{/each}
						{#if (ssh.keys ?? []).length === 0}<p class="text-muted-foreground text-xs">No keys installed.</p>{/if}
					</div>
					<div class="space-y-1.5">
						<Label for="nk">Add a public key</Label>
						<textarea id="nk" bind:value={newKey} rows="2" placeholder="ssh-ed25519 AAAA… you@host" class="border-input bg-background w-full rounded-md border px-2 py-1 font-mono text-xs"></textarea>
						<Button size="sm" disabled={busy === 'addkey' || !newKey.trim()} onclick={addKey}>Add key</Button>
					</div>
				{/if}
			</Card.Content>
		</Card.Root>

		<Card.Root>
			<Card.Header>
				<Card.Title class="flex items-center gap-2">
					<Download class="size-5" /> Software Update
					{#if upd?.update_available}<Badge class="ml-auto">Update available</Badge>{/if}
				</Card.Title>
			</Card.Header>
			<Card.Content class="space-y-3">
				{#if upd}
					<p class="text-muted-foreground text-sm">
						Current <code>{upd.current_version}</code>{#if upd.latest_version} · latest <code>{upd.latest_version}</code>{/if}
					</p>
				{/if}
				<div class="flex flex-wrap items-center gap-2">
					<Button variant="secondary" disabled={checking} onclick={checkUpdate}>{checking ? 'Checking…' : 'Check for updates'}</Button>
					<Button disabled={applying || !upd?.update_available} onclick={applyUpdate}>{applying ? 'Updating…' : 'Update now'}</Button>
					{#if upd?.state === 'error' && upd.error}<span class="text-destructive text-xs">{upd.error}</span>{/if}
				</div>
			</Card.Content>
		</Card.Root>
		<Card.Root>
			<Card.Header>
				<Card.Title class="flex items-center gap-2">
					<KeyRound class="size-5" /> Admin Password
				</Card.Title>
			</Card.Header>
			<Card.Content class="space-y-4">
				<div class="space-y-1.5">
					<Label for="cp">Current password</Label>
					<Input id="cp" type="password" bind:value={currentPassword} autocomplete="current-password" />
				</div>
				<div class="grid gap-4 sm:grid-cols-2">
					<div class="space-y-1.5">
						<Label for="np">New password</Label>
						<Input id="np" type="password" bind:value={newPassword} autocomplete="new-password" />
					</div>
					<div class="space-y-1.5">
						<Label for="np2">Confirm new password</Label>
						<Input id="np2" type="password" bind:value={confirmPassword} autocomplete="new-password" />
					</div>
				</div>
				<p class="text-muted-foreground text-xs">At least 8 characters.</p>
			</Card.Content>
			<Card.Footer>
				<Button onclick={changePassword} disabled={changingPassword || !passwordValid}>
					{changingPassword ? 'Saving…' : 'Change password'}
				</Button>
			</Card.Footer>
		</Card.Root>
	</div>
</main>
