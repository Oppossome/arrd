<script lang="ts" generics="P extends Record<string, unknown>">
	import type { Component } from 'svelte';
	import { QueryClientProvider, type QueryClient } from '@tanstack/svelte-query';

	interface Props {
		client: QueryClient;
		component: Component<P>;
		props: P;
		/** Receives the inner component instance (for its exported functions). */
		onref?: (instance: unknown) => void;
	}
	let { client, component: Inner, props, onref }: Props = $props();
	let ref = $state<unknown>();
	$effect(() => {
		if (ref) onref?.(ref);
	});
</script>

<QueryClientProvider {client}>
	<Inner bind:this={ref} {...props} />
</QueryClientProvider>
