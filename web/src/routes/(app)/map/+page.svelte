<script lang="ts">
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import * as api from '$lib/sdk';
	import type { MapDto, RoomGeomDto, ZoneDto } from '$lib/sdk';

	import * as Dialog from '$lib/components/ui/dialog';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import Scissors from '@lucide/svelte/icons/scissors';
	import Combine from '@lucide/svelte/icons/combine';
	import Pencil from '@lucide/svelte/icons/pencil';
	import Trash2 from '@lucide/svelte/icons/trash-2';
	import Plus from '@lucide/svelte/icons/plus';
	import Move from '@lucide/svelte/icons/move';
	import Check from '@lucide/svelte/icons/check';
	import X from '@lucide/svelte/icons/x';

	let map = $state<MapDto | null>(null);
	let selected = $state<Record<string, boolean>>({});
	let busy = $state(false);
	let svgEl = $state<SVGSVGElement | null>(null);

	let draw = $state<{ type: 'split'; roomId: string } | { type: 'zone'; kind: string } | null>(null);
	let pts = $state<[number, number][]>([]);
	let drawName = $state('');

	let edit = $state<{ id: string; kind: string; name: string; x0: number; y0: number; x1: number; y1: number } | null>(null);
	let drag: { type: 'move' | 'corner'; which?: string; start?: [number, number]; startRect?: { x0: number; y0: number; x1: number; y1: number } } | null = null;

	let renameOpen = $state(false);
	let renameValue = $state('');
	let mergeOpen = $state(false);
	let mergeName = $state('');

	const palette = ['#3b82f6', '#22c55e', '#f59e0b', '#a855f7', '#ef4444'];
	const zoneStyle: Record<string, { c: string; label: string }> = {
		carpet: { c: '#a855f7', label: 'Carpet' },
		no_go: { c: '#ef4444', label: 'No-Go' },
		no_mop: { c: '#0ea5e9', label: 'No-Mop' },
		base: { c: '#64748b', label: 'Dock' },
		other: { c: '#64748b', label: 'Other' }
	};

	let rooms = $derived<RoomGeomDto[]>(map?.rooms ?? []);
	let zones = $derived<ZoneDto[]>(map?.zones ?? []);
	let editableZones = $derived(zones.filter((z) => z.kind !== 'base' && z.kind !== 'other'));

	function zoneLabel(z: ZoneDto): string {
		const isDefault = !z.name || [...z.name].some((c) => c.charCodeAt(0) > 127);
		if (!isDefault) return z.name;
		const label = zoneStyle[z.kind].label;
		const sameKind = editableZones.filter((x) => x.kind === z.kind);
		if (sameKind.length <= 1) return label;
		return `${label} ${sameKind.findIndex((x) => x.id === z.id) + 1}`;
	}

	let selectedRoomIds = $derived(rooms.filter((r) => selected[r.id]).map((r) => r.id));
	let selectedZoneIds = $derived(editableZones.filter((z) => selected[z.id]).map((z) => z.id));
	let oneRoom = $derived(selectedRoomIds.length === 1 ? rooms.find((r) => r.id === selectedRoomIds[0])! : null);
	let oneZone = $derived(selectedZoneIds.length === 1 ? editableZones.find((z) => z.id === selectedZoneIds[0])! : null);
	let panelOpen = $derived(!!(draw || edit || selectedRoomIds.length || selectedZoneIds.length));

	function w2px(x: number) { return map ? (x - map.origin_x) / map.resolution : 0; }
	function w2py(y: number) { return map ? map.height - (y - map.origin_y) / map.resolution : 0; }
	function px2wx(col: number) { return map ? map.origin_x + col * map.resolution : 0; }
	function px2wy(row: number) { return map ? map.origin_y + (map.height - row) * map.resolution : 0; }
	function points(g: number[] | null) {
		const a = g ?? [];
		let s = '';
		for (let i = 0; i + 1 < a.length; i += 2) s += `${w2px(a[i]).toFixed(2)},${w2py(a[i + 1]).toFixed(2)} `;
		return s.trim();
	}
	function centroid(g: number[] | null): [number, number] {
		const a = g ?? [];
		let x = 0, y = 0, n = 0;
		for (let i = 0; i + 1 < a.length; i += 2) { x += w2px(a[i]); y += w2py(a[i + 1]); n++; }
		return n ? [x / n, y / n] : [0, 0];
	}
	let viewBox = $derived(map ? `0 0 ${map.width} ${map.height}` : '0 0 100 100');

	async function load() {
		try { map = await api.getMap(); } catch (e) { toast.error(`Failed to load map: ${e}`); }
	}

	function pick(id: string, e: MouseEvent) {
		if (draw || edit) return;
		if (e.metaKey || e.ctrlKey || e.shiftKey) selected[id] = !selected[id];
		else selected = { [id]: true };
	}

	function svgPt(evt: PointerEvent): [number, number] {
		const p = svgEl!.createSVGPoint();
		p.x = evt.clientX; p.y = evt.clientY;
		const m = p.matrixTransform(svgEl!.getScreenCTM()!.inverse());
		return [m.x, m.y];
	}
	function svgWorld(evt: PointerEvent): [number, number] {
		const [c, r] = svgPt(evt);
		return [px2wx(c), px2wy(r)];
	}

	function onSvgDown(evt: PointerEvent) {
		if (draw) {
			if (pts.length >= 2) pts = [];
			pts = [...pts, svgPt(evt)];
			return;
		}
		if (edit) return;
		const tag = (evt.target as Element).tagName;
		if (tag === 'svg' || tag === 'image') selected = {};
	}
	function onSvgMove(evt: PointerEvent) {
		if (!edit || !drag) return;
		const [wx, wy] = svgWorld(evt);
		if (drag.type === 'move' && drag.start && drag.startRect) {
			const dx = wx - drag.start[0], dy = wy - drag.start[1];
			edit = { ...edit, x0: drag.startRect.x0 + dx, y0: drag.startRect.y0 + dy, x1: drag.startRect.x1 + dx, y1: drag.startRect.y1 + dy };
		} else if (drag.type === 'corner') {
			const w = drag.which!;
			edit = { ...edit, [w.includes('x0') ? 'x0' : 'x1']: wx, [w.includes('y0') ? 'y0' : 'y1']: wy } as typeof edit;
		}
	}
	function onSvgUp(evt: PointerEvent) {
		if (drag) { drag = null; svgEl?.releasePointerCapture(evt.pointerId); }
	}
	function cornerDown(which: string, e: PointerEvent) {
		e.stopPropagation();
		drag = { type: 'corner', which };
		svgEl?.setPointerCapture(e.pointerId);
	}
	function rectDown(e: PointerEvent) {
		e.stopPropagation();
		drag = { type: 'move', start: svgWorld(e), startRect: { x0: edit!.x0, y0: edit!.y0, x1: edit!.x1, y1: edit!.y1 } };
		svgEl?.setPointerCapture(e.pointerId);
	}

	async function run(label: string, fn: () => Promise<unknown>, ok: string) {
		busy = true;
		try { await fn(); toast.success(ok); selected = {}; await load(); }
		catch (e) { toast.error(`${label} failed: ${e}`); }
		finally { busy = false; }
	}

	function doRename() {
		if (!oneRoom) return;
		const id = oneRoom.id; renameOpen = false;
		run('Rename', () => api.renameRoom({ id, name: renameValue }), 'Room renamed');
	}
	function doMerge() {
		const ids = [...selectedRoomIds]; mergeOpen = false;
		run('Merge', () => api.mergeRooms({ ids, name: mergeName }), 'Rooms merged');
	}
	function startSplit() {
		if (!oneRoom) return;
		draw = { type: 'split', roomId: oneRoom.id }; pts = []; drawName = '';
	}
	function startZone(kind: string) {
		draw = { type: 'zone', kind }; pts = []; drawName = ''; selected = {};
	}
	function cancelDraw() { draw = null; pts = []; }

	function closePanel() {
		if (edit) cancelEdit();
		else if (draw) cancelDraw();
		else selected = {};
	}
	function onKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') closePanel();
	}
	function confirmDraw() {
		if (!draw || pts.length < 2) return;
		if (draw.type === 'split') {
			const id = draw.roomId;
			const line: [number, number, number, number] = [px2wx(pts[0][0]), px2wy(pts[0][1]), px2wx(pts[1][0]), px2wy(pts[1][1])];
			const nm = drawName; cancelDraw();
			run('Split', () => api.splitRoom({ id, line, new_name: nm }), 'Room split');
		} else {
			const x0 = px2wx(pts[0][0]), y0 = px2wy(pts[0][1]), x1 = px2wx(pts[1][0]), y1 = px2wy(pts[1][1]);
			const geometry = rectGeom(x0, y0, x1, y1);
			const kind = draw.kind, nm = drawName; cancelDraw();
			run('Add zone', () => api.addZone({ kind, geometry, name: nm }), 'Zone added');
		}
	}

	function rectGeom(x0: number, y0: number, x1: number, y1: number) {
		const minx = Math.min(x0, x1), maxx = Math.max(x0, x1), miny = Math.min(y0, y1), maxy = Math.max(y0, y1);
		return [minx, miny, maxx, miny, maxx, maxy, minx, maxy];
	}

	function startEdit(z: ZoneDto) {
		const g = z.geometry ?? [];
		const xs = g.filter((_, i) => i % 2 === 0), ys = g.filter((_, i) => i % 2 === 1);
		selected = { [z.id]: true };
		draw = null;
		edit = { id: z.id, kind: z.kind, name: zoneLabel(z), x0: Math.min(...xs), y0: Math.min(...ys), x1: Math.max(...xs), y1: Math.max(...ys) };
	}
	function cancelEdit() { edit = null; drag = null; }
	function saveEdit() {
		if (!edit) return;
		const { id, name } = edit;
		const geometry = rectGeom(edit.x0, edit.y0, edit.x1, edit.y1);
		edit = null;
		run('Save zone', () => api.updateZone({ id, name, geometry }), 'Zone updated');
	}
	function deleteZone(id: string) {
		edit = null;
		run('Delete', () => api.deleteZone({ id }), 'Zone deleted');
	}
	function deleteSelectedZones() {
		const ids = [...selectedZoneIds];
		run('Delete', async () => { for (const id of ids) await api.deleteZone({ id }); }, `Deleted ${ids.length} zone${ids.length > 1 ? 's' : ''}`);
	}

	let rectPreview = $derived.by(() => {
		if (draw?.type !== 'zone' || pts.length < 2) return null;
		const [a, b] = pts;
		return { x: Math.min(a[0], b[0]), y: Math.min(a[1], b[1]), w: Math.abs(a[0] - b[0]), h: Math.abs(a[1] - b[1]) };
	});

	let editRect = $derived.by(() => {
		if (!edit) return null;
		const px0 = w2px(edit.x0), px1 = w2px(edit.x1), py0 = w2py(edit.y0), py1 = w2py(edit.y1);
		return {
			x: Math.min(px0, px1), y: Math.min(py0, py1), w: Math.abs(px1 - px0), h: Math.abs(py1 - py0),
			handles: [
				{ which: 'x0y0', cx: px0, cy: py0 },
				{ which: 'x1y0', cx: px1, cy: py0 },
				{ which: 'x1y1', cx: px1, cy: py1 },
				{ which: 'x0y1', cx: px0, cy: py1 }
			]
		};
	});

	onMount(load);
</script>

<svelte:window onkeydown={onKeydown} />

<div class="flex h-[calc(100vh-3.5rem)]">
	<aside class="flex w-72 shrink-0 flex-col overflow-y-auto border-r">
		<div class="space-y-1 p-3">
			<h2 class="text-muted-foreground px-1 text-xs font-semibold uppercase tracking-wide">Rooms</h2>
			{#each rooms as r (r.id)}
				<div class="group flex items-center gap-1 rounded-md pr-1 text-sm {selected[r.id] ? 'bg-secondary' : 'hover:bg-accent'}">
					<button type="button" onclick={(e) => pick(r.id, e)} class="flex flex-1 items-center gap-2 truncate px-2 py-1.5 text-left">
						<span class="size-3 shrink-0 rounded-sm" style="background: {palette[r.color_type % palette.length]}"></span>
						<span class="flex-1 truncate">{r.name}</span>
					</button>
					<button type="button" title="Edit" onclick={() => (selected = { [r.id]: true })} class="text-muted-foreground hover:text-foreground rounded p-1 opacity-0 group-hover:opacity-100 {selected[r.id] ? 'opacity-100' : ''}">
						<Pencil class="size-3.5" />
					</button>
				</div>
			{/each}
		</div>

		<div class="space-y-1 border-t p-3">
			<div class="flex items-center justify-between px-1">
				<h2 class="text-muted-foreground text-xs font-semibold uppercase tracking-wide">Zones</h2>
				<DropdownMenu.Root>
					<DropdownMenu.Trigger disabled={busy} title="Add zone" class="text-muted-foreground hover:text-foreground rounded p-0.5">
						<Plus class="size-4" />
					</DropdownMenu.Trigger>
					<DropdownMenu.Content align="end" class="w-36">
						<DropdownMenu.Item onSelect={() => startZone('carpet')}>Carpet</DropdownMenu.Item>
						<DropdownMenu.Item onSelect={() => startZone('no_go')}>No-Go zone</DropdownMenu.Item>
						<DropdownMenu.Item onSelect={() => startZone('no_mop')}>No-Mop zone</DropdownMenu.Item>
					</DropdownMenu.Content>
				</DropdownMenu.Root>
			</div>
			{#each editableZones as z (z.id)}
				<div class="group flex items-center gap-1 rounded-md pr-1 text-sm {selected[z.id] ? 'bg-secondary' : 'hover:bg-accent'}">
					<button type="button" onclick={(e) => pick(z.id, e)} class="flex flex-1 items-center gap-2 truncate px-2 py-1.5 text-left">
						<span class="size-3 shrink-0 rounded-full" style="background: {zoneStyle[z.kind].c}"></span>
						<span class="truncate">{zoneLabel(z)}</span>
					</button>
					<button type="button" title="Edit shape" onclick={() => startEdit(z)} class="text-muted-foreground hover:text-foreground rounded p-1 opacity-0 group-hover:opacity-100 {selected[z.id] ? 'opacity-100' : ''}">
						<Pencil class="size-3.5" />
					</button>
				</div>
			{/each}
			{#if editableZones.length === 0}
				<p class="text-muted-foreground px-1 py-1 text-xs">No zones yet. Use + to add one.</p>
			{/if}
		</div>
	</aside>

	<div class="relative flex-1 overflow-hidden bg-muted/30">
		{#if map}
			<svg
				bind:this={svgEl}
				data-testid="map-svg"
				viewBox={viewBox}
				preserveAspectRatio="xMidYMid meet"
				class="h-full w-full touch-none select-none {draw ? 'cursor-crosshair' : ''}"
				style="image-rendering: pixelated;"
				onpointerdown={onSvgDown}
				onpointermove={onSvgMove}
				onpointerup={onSvgUp}
				role="presentation"
			>
				<image href={map.image_png} x="0" y="0" width={map.width} height={map.height} opacity="0.5" />

				{#each rooms as r (r.id)}
					<polygon
						points={points(r.geometry)}
						fill={palette[r.color_type % palette.length]}
						fill-opacity={selected[r.id] ? 0.6 : 0.3}
						stroke={palette[r.color_type % palette.length]}
						stroke-width={selected[r.id] ? 1.4 : 0.5}
						class="outline-none {draw || edit ? '' : 'cursor-pointer'}"
						role="button" tabindex="-1"
						onclick={(e) => pick(r.id, e)} onkeydown={() => {}}
					/>
				{/each}

				{#each zones as z (z.id)}
					{#if z.kind === 'base'}
						{@const c = centroid(z.geometry)}
						<circle cx={c[0]} cy={c[1]} r="2.2" fill={zoneStyle.base.c} stroke="white" stroke-width="0.6" />
					{:else if edit?.id !== z.id}
						<polygon
							points={points(z.geometry)}
							fill={zoneStyle[z.kind].c}
							fill-opacity={selected[z.id] ? 0.5 : 0.25}
							stroke={zoneStyle[z.kind].c}
							stroke-width={selected[z.id] ? 1.2 : 0.6}
							stroke-dasharray={z.kind === 'no_go' ? '2 1.2' : ''}
							class="outline-none {draw || edit ? '' : 'cursor-pointer'}"
							role="button" tabindex="-1"
							onclick={(e) => pick(z.id, e)} onkeydown={() => {}}
						/>
					{/if}
				{/each}

				{#each rooms as r (r.id + '-l')}
					{@const c = centroid(r.geometry)}
					<text x={c[0]} y={c[1]} text-anchor="middle" class="pointer-events-none fill-foreground" style="font-size: 6px; paint-order: stroke; stroke: var(--background); stroke-width: 1.5px;">{r.name}</text>
				{/each}

				{#if editRect && edit}
					<rect x={editRect.x} y={editRect.y} width={editRect.w} height={editRect.h}
						fill={zoneStyle[edit.kind].c} fill-opacity="0.4" stroke={zoneStyle[edit.kind].c} stroke-width="1.2"
						class="cursor-move" role="presentation" onpointerdown={rectDown} />
					{#each editRect.handles as h (h.which)}
						<circle cx={h.cx} cy={h.cy} r="2.4" fill="white" stroke={zoneStyle[edit.kind].c} stroke-width="1"
							class="cursor-pointer" role="presentation" onpointerdown={(e) => cornerDown(h.which, e)} />
					{/each}
				{/if}

				{#if draw?.type === 'split' && pts.length === 2}
					<line x1={pts[0][0]} y1={pts[0][1]} x2={pts[1][0]} y2={pts[1][1]} stroke="#ef4444" stroke-width="1.2" stroke-dasharray="2 1.5" />
				{/if}
				{#if rectPreview}
					<rect x={rectPreview.x} y={rectPreview.y} width={rectPreview.w} height={rectPreview.h} fill={zoneStyle[(draw as { kind: string }).kind].c} fill-opacity="0.35" stroke={zoneStyle[(draw as { kind: string }).kind].c} stroke-width="1" />
				{/if}
				{#each pts as p, i (i)}
					<circle cx={p[0]} cy={p[1]} r="1.4" fill="#ef4444" />
				{/each}
			</svg>

			{#if panelOpen}
				<div class="bg-background/95 absolute left-3 top-3 w-60 space-y-2 rounded-lg border p-3 text-sm shadow-lg">
					<button type="button" title="Close (Esc)" onclick={closePanel} class="text-muted-foreground hover:text-foreground absolute right-1.5 top-1.5 rounded p-1">
						<X class="size-4" />
					</button>
					{#if draw}
						<p class="pr-6 font-medium">{draw.type === 'split' ? 'Split room' : `Add ${zoneStyle[draw.kind].label} zone`}</p>
						<p class="text-muted-foreground text-xs">{draw.type === 'split' ? 'Click two points for the split line' : 'Click two opposite corners'} ({pts.length}/2)</p>
						<Input bind:value={drawName} placeholder={draw.type === 'split' ? 'New room name' : 'Zone name (optional)'} />
						<Button size="sm" class="w-full" disabled={busy || pts.length < 2} onclick={confirmDraw}><Check class="size-4" /> Confirm</Button>
					{:else if edit}
						<p class="flex items-center gap-1.5 font-medium"><Move class="size-4" /> Edit zone</p>
						<p class="text-muted-foreground text-xs">Drag the box to move, corners to resize.</p>
						<Input bind:value={edit.name} placeholder="Zone name" />
						<Button size="sm" class="w-full" disabled={busy} onclick={saveEdit}><Check class="size-4" /> Save</Button>
						<Button size="sm" variant="outline" class="text-destructive w-full" disabled={busy} onclick={() => deleteZone(edit!.id)}><Trash2 class="size-4" /> Delete zone</Button>
					{:else}
						<p class="text-muted-foreground text-xs">{selectedRoomIds.length + selectedZoneIds.length} selected</p>
						{#if oneRoom}
							<Button size="sm" variant="secondary" class="w-full justify-start" disabled={busy} onclick={() => { renameValue = oneRoom!.name; renameOpen = true; }}><Pencil class="size-4" /> Rename room</Button>
							<Button size="sm" variant="secondary" class="w-full justify-start" disabled={busy} onclick={startSplit}><Scissors class="size-4" /> Split room</Button>
						{/if}
						{#if selectedRoomIds.length >= 2}
							<Button size="sm" variant="secondary" class="w-full justify-start" disabled={busy} onclick={() => { mergeName = ''; mergeOpen = true; }}><Combine class="size-4" /> Merge {selectedRoomIds.length} rooms</Button>
						{/if}
						{#if oneZone}
							<Button size="sm" variant="secondary" class="w-full justify-start" disabled={busy} onclick={() => startEdit(oneZone!)}><Move class="size-4" /> Edit zone shape</Button>
						{/if}
						{#if selectedZoneIds.length >= 1}
							<Button size="sm" variant="secondary" class="text-destructive w-full justify-start" disabled={busy} onclick={deleteSelectedZones}><Trash2 class="size-4" /> Delete {selectedZoneIds.length} zone{selectedZoneIds.length > 1 ? 's' : ''}</Button>
						{/if}
					{/if}
				</div>
			{/if}
		{:else}
			<p class="text-muted-foreground p-8 text-center text-sm">Loading map…</p>
		{/if}
	</div>
</div>

<Dialog.Root bind:open={renameOpen}>
	<Dialog.Content>
		<Dialog.Header><Dialog.Title>Rename room</Dialog.Title></Dialog.Header>
		<div class="space-y-1.5"><Label for="rn">Name</Label><Input id="rn" bind:value={renameValue} /></div>
		<Dialog.Footer>
			<Button variant="ghost" onclick={() => (renameOpen = false)}>Cancel</Button>
			<Button onclick={doRename}>Save</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<Dialog.Root bind:open={mergeOpen}>
	<Dialog.Content>
		<Dialog.Header><Dialog.Title>Merge {selectedRoomIds.length} rooms</Dialog.Title></Dialog.Header>
		<div class="space-y-1.5"><Label for="mn">Merged room name</Label><Input id="mn" bind:value={mergeName} placeholder="(keeps first room's name)" /></div>
		<Dialog.Footer>
			<Button variant="ghost" onclick={() => (mergeOpen = false)}>Cancel</Button>
			<Button onclick={doMerge}>Merge</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
