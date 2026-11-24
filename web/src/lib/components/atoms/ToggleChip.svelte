<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		/** Whether the chip's filter is on. */
		pressed: boolean;
		/** How many results the chip stands for, shown after its label. */
		count?: number;
		onclick: () => void;
		children: Snippet;
	}
	let { pressed, count, onclick, children }: Props = $props();
</script>

<button type="button" class="chip" class:pressed aria-pressed={pressed} {onclick}>
	{@render children()}
	{#if count !== undefined}<span class="count">{count}</span>{/if}
</button>

<style>
	.chip {
		display: inline-flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-1) var(--space-3);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-pill);
		background: var(--color-surface);
		color: var(--color-text-muted);
		font: inherit;
		font-size: var(--text-xs);
		white-space: nowrap;
		cursor: pointer;
		transition:
			background var(--transition-fast),
			color var(--transition-fast),
			border-color var(--transition-fast);
	}
	.chip:hover {
		background: var(--color-surface-raised);
		color: var(--color-text);
	}
	.pressed {
		border-color: var(--color-accent);
		background: var(--color-surface-raised);
		color: var(--color-text);
	}
	.count {
		color: var(--color-text-muted);
		font-variant-numeric: tabular-nums;
	}
</style>
