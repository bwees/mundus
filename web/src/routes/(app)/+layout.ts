import { authenticate } from '$lib/utils/auth';
import type { LayoutLoad } from './$types';

export const load: LayoutLoad = async () => {
	await authenticate();
};
