<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Text } from '#lib/components/atoms/index.js';

	interface Props {
		message: string;
		action?: Snippet;
	}
	let { message, action }: Props = $props();
</script>

<div class="empty">
	<Text tone="muted">{message}</Text>
	{@render action?.()}
</div>

<style>
	.empty {
		display: grid;
		justify-items: center;
		gap: var(--space-3);
		padding: var(--space-7) var(--space-4);
		border: 1px dashed var(--color-border);
		border-radius: var(--radius-lg);
		text-align: center;
	}
</style>
