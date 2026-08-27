<script lang="ts">
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import * as api from '$lib/sdk';
	import type { StateDto, RoomDto, SettingsDto, Option } from '$lib/sdk';

	import * as Card from '$lib/components/ui/card';
	import * as Select from '$lib/components/ui/select';
	import { Button } from '$lib/components/ui/button';
	import { Switch } from '$lib/components/ui/switch';
	import { Slider } from '$lib/components/ui/slider';
	import { Label } from '$lib/components/ui/label';
	import { Badge } from '$lib/components/ui/badge';
	import { Separator } from '$lib/components/ui/separator';

	import Play from '@lucide/svelte/icons/play';
	import Pause from '@lucide/svelte/icons/pause';
	import Square from '@lucide/svelte/icons/square';
	import House from '@lucide/svelte/icons/house';
	import MapPin from '@lucide/svelte/icons/map-pin';
	import Droplets from '@lucide/svelte/icons/droplets';
	import Trash2 from '@lucide/svelte/icons/trash-2';
	import Wind from '@lucide/svelte/icons/wind';
	import Battery from '@lucide/svelte/icons/battery-medium';

	let robot = $state<StateDto | null>(null);
	let rooms = $state<RoomDto[]>([]);
	let selected = $state<Record<string, boolean>>({});
	let settings = $state<SettingsDto | null>(null);

	let toggles = $derived(settings?.schema?.filter((s) => s.kind === 'toggle') ?? []);
	let choices = $derived(settings?.schema?.filter((s) => s.kind === 'choice') ?? []);
	let numbers = $derived(settings?.schema?.filter((s) => s.kind === 'number') ?? []);
	let busy = $state('');

	let selectedCount = $derived(rooms.filter((r) => selected[r.id]).length);

	let mType = $state('sweep_mop');
	let mFan = $state('2');
	let mWater = $state('1');
	let mTimes = $state('1');

	const typeOpts = [
		{ v: 'sweep', l: 'Vacuum' },
		{ v: 'mop', l: 'Mop' },
		{ v: 'sweep_mop', l: 'Vacuum + Mop' },
		{ v: 'first_sweep_then_mop', l: 'Vacuum then Mop' }
	];
	const fanOpts = [
		{ v: '1', l: 'Quiet' },
		{ v: '2', l: 'Standard' },
		{ v: '3', l: 'Strong' },
		{ v: '4', l: 'Max' }
	];
	const waterOpts = [
		{ v: '1', l: 'Low' },
		{ v: '2', l: 'Medium' },
		{ v: '3', l: 'High' }
	];
	const timesOpts = [
		{ v: '1', l: '1 Pass' },
		{ v: '2', l: '2 Passes' }
	];
	function mode() {
		return { type: mType, fan_level: +mFan, water_level: +mWater, times: +mTimes };
	}

	async function refresh() {
		try {
			robot = await api.getState();
		} catch (e) {
			robot = null;
		}
	}

	async function act(label: string, fn: () => Promise<unknown>, ok: string) {
		busy = label;
		try {
			await fn();
			toast.success(ok);
		} catch (e) {
			toast.error(`${ok} failed: ${e}`);
		} finally {
			busy = '';
			await refresh();
		}
	}

	function cleanSelected() {
		const ids = rooms.filter((r) => selected[r.id]).map((r) => r.id);
		if (ids.length === 0) return;
		return act('clean', () => api.startClean({ rooms: ids, mode: mode() }), 'Cleaning selected rooms');
	}

	onMount(() => {
		refresh();
		(async () => {
			try {
				rooms = await api.getRooms();
			} catch {}
			try {
				settings = await api.getSettings();
			} catch {}
		})();
		const t = setInterval(refresh, 3000);
		return () => clearInterval(t);
	});

	function stateVariant(s?: string): 'default' | 'secondary' | 'destructive' | 'outline' {
		if (s === 'error') return 'destructive';
		if (s === 'cleaning') return 'default';
		return 'secondary';
	}
</script>

{#snippet numSelect(value: number, set: (n: number) => void, options: Option[])}
	<Select.Root type="single" value={String(value)} onValueChange={(v) => set(Number(v))}>
		<Select.Trigger class="w-full">
			{options.find((o) => o.value === value)?.label ?? '-'}
		</Select.Trigger>
		<Select.Content>
			{#each options as o (o.value)}
				<Select.Item value={String(o.value)} label={o.label}>{o.label}</Select.Item>
			{/each}
		</Select.Content>
	</Select.Root>
{/snippet}

{#snippet strSelect(value: string, set: (s: string) => void, options: { v: string; l: string }[])}
	<Select.Root type="single" {value} onValueChange={set}>
		<Select.Trigger class="w-full">
			{options.find((o) => o.v === value)?.l ?? '—'}
		</Select.Trigger>
		<Select.Content>
			{#each options as o (o.v)}
				<Select.Item value={o.v} label={o.l}>{o.l}</Select.Item>
			{/each}
		</Select.Content>
	</Select.Root>
{/snippet}

{#snippet toggle(label: string, checked: boolean, set: (b: boolean) => void)}
	<div class="flex items-center justify-between gap-4 py-1">
		<Label class="font-normal">{label}</Label>
		<Switch {checked} onCheckedChange={set} />
	</div>
{/snippet}

<main>
	<div class="mx-auto max-w-4xl space-y-6 p-4 sm:p-6">
		<header class="flex items-center gap-3">
			<h1 class="text-2xl font-bold tracking-tight">{robot?.device_name || 'SwitchBot S20'}</h1>
			{#if robot}
				<Badge variant={stateVariant(robot.state)} class="capitalize">{robot.state}</Badge>
			{:else}
				<Badge variant="outline">Offline</Badge>
			{/if}
		</header>

		{#if robot}
			<Card.Root>
				<Card.Content class="grid grid-cols-2 gap-4 sm:grid-cols-4">
					<div class="flex items-center gap-3">
						<Battery class="text-muted-foreground size-5" />
						<div>
							<div class="text-muted-foreground text-xs">Battery</div>
							<div class="text-lg font-semibold">{robot.battery_level}%</div>
						</div>
					</div>
					<div class="flex items-center gap-3">
						<Wind class="text-muted-foreground size-5" />
						<div>
							<div class="text-muted-foreground text-xs">Fan</div>
							<div class="text-lg font-semibold capitalize">{robot.fan_speed || '—'}</div>
						</div>
					</div>
					<div>
						<div class="text-muted-foreground text-xs">Charging</div>
						<div class="text-lg font-semibold">{robot.charging ? 'Yes' : 'No'}</div>
					</div>
					<div>
						<div class="text-muted-foreground text-xs">Error</div>
						<div class="text-lg font-semibold">{robot.error_code || 'None'}</div>
					</div>
				</Card.Content>
			</Card.Root>
		{/if}

		<Card.Root>
			<Card.Header>
				<Card.Title>Controls</Card.Title>
			</Card.Header>
			<Card.Content class="flex flex-wrap gap-2">
				<Button disabled={!!busy} onclick={() => act('start', () => api.startClean({ rooms: [], mode: mode() }), 'Cleaning started')}>
					<Play class="size-4" /> Clean All
				</Button>
				<Button variant="secondary" disabled={!!busy} onclick={() => act('pause', () => api.pause(), 'Paused')}>
					<Pause class="size-4" /> Pause
				</Button>
				<Button variant="secondary" disabled={!!busy} onclick={() => act('resume', () => api.resume(), 'Resumed')}>
					<Play class="size-4" /> Resume
				</Button>
				<Button variant="secondary" disabled={!!busy} onclick={() => act('stop', () => api.stop(), 'Stopped')}>
					<Square class="size-4" /> Stop
				</Button>
				<Button variant="secondary" disabled={!!busy} onclick={() => act('dock', () => api.dock(), 'Returning to dock')}>
					<House class="size-4" /> Dock
				</Button>
				<Button variant="outline" disabled={!!busy} onclick={() => act('locate', () => api.locate(), 'Locating')}>
					<MapPin class="size-4" /> Locate
				</Button>
			</Card.Content>
		</Card.Root>

		<Card.Root>
			<Card.Header>
				<Card.Title>Clean Mode</Card.Title>
			</Card.Header>
			<Card.Content class="grid grid-cols-2 gap-4 sm:grid-cols-4">
				<div class="space-y-1.5">
					<Label>Type</Label>
					{@render strSelect(mType, (v) => (mType = v), typeOpts)}
				</div>
				<div class="space-y-1.5">
					<Label>Fan</Label>
					{@render strSelect(mFan, (v) => (mFan = v), fanOpts)}
				</div>
				<div class="space-y-1.5">
					<Label>Water</Label>
					{@render strSelect(mWater, (v) => (mWater = v), waterOpts)}
				</div>
				<div class="space-y-1.5">
					<Label>Passes</Label>
					{@render strSelect(mTimes, (v) => (mTimes = v), timesOpts)}
				</div>
			</Card.Content>
		</Card.Root>

		<Card.Root>
			<Card.Header>
				<Card.Title>Rooms</Card.Title>
				<Card.Action>
					<Button size="sm" disabled={!!busy || selectedCount === 0} onclick={cleanSelected}>Clean Selected</Button>
				</Card.Action>
			</Card.Header>
			<Card.Content class="flex flex-wrap gap-2">
				{#each rooms as r (r.id)}
					<button
						type="button"
						onclick={() => (selected[r.id] = !selected[r.id])}
						class="rounded-full border px-3 py-1 text-sm transition-colors {selected[r.id]
							? 'bg-primary text-primary-foreground border-primary'
							: 'bg-background hover:bg-accent'}"
					>
						{r.name}
					</button>
				{/each}
				{#if rooms.length === 0}
					<span class="text-muted-foreground text-sm">No rooms mapped yet.</span>
				{/if}
			</Card.Content>
		</Card.Root>

		<Card.Root>
			<Card.Header>
				<Card.Title>Base Station</Card.Title>
			</Card.Header>
			<Card.Content class="flex flex-wrap gap-2">
				<Button variant="secondary" disabled={!!busy} onclick={() => act('wash', () => api.selfClean({ action: 1 }), 'Washing mop')}>
					<Droplets class="size-4" /> Wash Mop
				</Button>
				<Button variant="secondary" disabled={!!busy} onclick={() => act('dust', () => api.selfClean({ action: 4 }), 'Emptying dust')}>
					<Trash2 class="size-4" /> Empty Dust
				</Button>
				<Button variant="secondary" disabled={!!busy} onclick={() => act('dry', () => api.selfClean({ action: 2 }), 'Drying started')}>
					<Wind class="size-4" /> Start Drying
				</Button>
				<Button variant="outline" disabled={!!busy} onclick={() => act('stopdry', () => api.selfClean({ action: 3 }), 'Drying stopped')}>
					Stop Drying
				</Button>
			</Card.Content>
		</Card.Root>

		{#if settings}
			<Card.Root>
				<Card.Header>
					<Card.Title>Settings</Card.Title>
					<Card.Action>
						<Button size="sm" disabled={!!busy} onclick={() => act('settings', () => api.setSettings({ values: settings!.values }), 'Settings saved')}>
							Save
						</Button>
					</Card.Action>
				</Card.Header>
				<Card.Content class="space-y-4">
					<div class="grid gap-x-8 gap-y-1 sm:grid-cols-2">
						{#each toggles as s (s.key)}
							{@render toggle(s.name, settings.values[s.key] === 1, (b) => (settings!.values[s.key] = b ? 1 : 0))}
						{/each}
					</div>

					<Separator />

					<div class="grid grid-cols-2 gap-4 sm:grid-cols-3">
						{#each choices as s (s.key)}
							<div class="space-y-1.5">
								<Label>{s.name}</Label>
								{@render numSelect(settings.values[s.key], (n) => (settings!.values[s.key] = n), s.options ?? [])}
							</div>
						{/each}
					</div>

					<Separator />

					{#each numbers as s (s.key)}
						<div>
							<div class="flex items-center justify-between">
								<Label>{s.name}</Label>
								<span class="text-muted-foreground text-sm">{settings.values[s.key]}</span>
							</div>
							<Slider
								type="single"
								value={settings.values[s.key]}
								onValueChange={(n) => (settings!.values[s.key] = n)}
								min={s.min ?? 0}
								max={s.max ?? 100}
								step={1}
								class="mt-4"
							/>
						</div>
					{/each}
				</Card.Content>
			</Card.Root>
		{/if}
	</div>
</main>
