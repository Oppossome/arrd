// The app's single QueryClient, wired to navigation and toasts. Shared by
// the root layout's load (auth gate) and its component (provider).

import { goto } from '$app/navigation';
import { createAppQueryClient } from './query';
import { toasts } from '../stores/toast.svelte';

export const queryClient = createAppQueryClient({
	onUnauthorized: () => {
		const here = location.pathname + location.search;
		if (location.pathname !== '/login') void goto(`/login?next=${encodeURIComponent(here)}`);
	},
	onSetupRequired: () => {
		if (location.pathname !== '/setup') void goto('/setup');
	},
	onMutationError: (message) => toasts.push('error', message)
});
