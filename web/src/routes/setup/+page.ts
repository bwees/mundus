import { requireSetup } from '$lib/utils/auth';
import type { PageLoad } from './$types';

export const load: PageLoad = async () => {
	await requireSetup();
};
