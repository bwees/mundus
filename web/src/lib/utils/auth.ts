import { goto } from '$app/navigation';
import { auth } from '$lib/managers/auth.svelte';
import { redirect } from '@sveltejs/kit';

export async function authenticate() {
	if (await auth.setupRequired()) redirect(302, '/setup');
	if (!(await auth.load())) redirect(302, '/login');
}

export async function requireGuest() {
	if (await auth.load()) redirect(302, '/');
}

export async function requireSetup() {
	if (!(await auth.setupRequired())) redirect(302, '/login');
}

export async function logoutAndRedirect() {
	auth.logout();
	await goto('/login');
}
