<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Input, Text } from '#lib/components/atoms/index.js';

	interface Props {
		/** Accessible name of the masked value. */
		label: string;
		/** Server-masked hint of the stored secret (e.g. "ab****zf"); empty when none is stored. */
		hint?: string;
		actions?: Snippet;
	}
	let { label, hint, actions }: Props = $props();
</script>

<div class="secret">
	<div class="value">
		{#if hint}
			<Input value={hint} readonly monospace tabindex={-1} aria-label={label} />
		{:else}
			<Text size="sm" tone="muted" as="span">Not set</Text>
		{/if}
	</div>
	{@render actions?.()}
</div>

<style>
	.secret {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}
	.value {
		flex: 1;
		min-width: 0;
	}
</style>
