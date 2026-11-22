<script lang="ts">
	import '#lib/styles/global.css';
	import { QueryClientProvider } from '@tanstack/svelte-query';
	import { queryClient } from '#lib/app-client.js';
	import { ToastHost } from '#lib/components/organisms/index.js';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();
</script>

<svelte:head><title>arrd</title></svelte:head>

<QueryClientProvider client={queryClient}>
	{@render children()}
	<ToastHost />
</QueryClientProvider>
