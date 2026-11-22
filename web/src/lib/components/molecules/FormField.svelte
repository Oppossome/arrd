<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Label, Text } from '#lib/components/atoms/index.js';

	interface Props {
		id: string;
		label: string;
		hint?: string;
		error?: string;
		/** Receives the id and aria attributes to put on the control. */
		children: Snippet<[{ id: string; 'aria-describedby'?: string; invalid: boolean }]>;
	}
	let { id, label, hint, error, children }: Props = $props();
	const describedBy = $derived(error ? `${id}-error` : hint ? `${id}-hint` : undefined);
</script>

<div class="field">
	<Label for={id}>{label}</Label>
	{@render children({ id, 'aria-describedby': describedBy, invalid: !!error })}
	{#if error}
		<span id="{id}-error"><Text tone="danger" size="xs" as="span">{error}</Text></span>
	{:else if hint}
		<span id="{id}-hint"><Text tone="muted" size="xs" as="span">{hint}</Text></span>
	{/if}
</div>

<style>
	.field {
		display: grid;
		gap: var(--space-1);
	}
</style>
