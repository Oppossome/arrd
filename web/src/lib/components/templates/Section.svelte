<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Heading, Text } from '#lib/components/atoms/index.js';

	interface Props {
		title: string;
		description?: string;
		children: Snippet;
	}
	let { title, description, children }: Props = $props();
</script>

<section class="section" aria-label={title}>
	<div class="head">
		<Heading level={2}>{title}</Heading>
		{#if description}<Text size="sm" tone="muted">{description}</Text>{/if}
	</div>
	{@render children()}
</section>

<style>
	.section {
		display: grid;
		gap: var(--space-3);
	}
	.head {
		display: grid;
		gap: var(--space-1);
	}
</style>
