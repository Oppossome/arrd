<script lang="ts">
	import type { Snippet } from 'svelte';
	interface Props {
		level?: 1 | 2 | 3;
		children: Snippet;
	}
	let { level = 1, children }: Props = $props();
</script>

<svelte:element this={`h${level}`} class="h{level}">{@render children()}</svelte:element>

<style>
	.h1,
	.h2,
	.h3 {
		margin: 0;
		line-height: var(--line-tight);
		font-weight: var(--weight-bold);
	}
	.h1 {
		font-size: var(--text-2xl);
	}
	.h2 {
		font-size: var(--text-xl);
	}
	.h3 {
		font-size: var(--text-lg);
	}
</style>
