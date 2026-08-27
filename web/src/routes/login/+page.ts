import { requireGuest } from '$lib/utils/auth';
import { auth } from '$lib/managers/auth.svelte';
import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const load: PageLoad = async () => {
	if (await auth.setupRequired()) redirect(302, '/setup');
	await requireGuest();
};
