<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Heading } from '#lib/components/atoms/index.js';

	interface Props {
		title: string;
		children: Snippet;
	}
	let { title, children }: Props = $props();
</script>

<div class="page">
	<section class="card" aria-label={title}>
		<Heading level={1}>{title}</Heading>
		{@render children()}
	</section>
</div>

<style>
	.page {
		display: grid;
		place-items: center;
		min-height: 100vh;
		padding: var(--space-4);
	}
	.card {
		display: grid;
		gap: var(--space-5);
		width: min(26rem, 100%);
		padding: var(--space-6);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-lg);
		background: var(--color-surface);
	}
</style>
