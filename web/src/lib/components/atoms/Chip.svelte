<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		href: string;
		/** Marks the chip as the current choice. */
		active?: boolean;
		children: Snippet;
	}
	let { href, active = false, children }: Props = $props();
</script>

<a class="chip" class:active {href} aria-current={active ? 'true' : undefined}>{@render children()}</a>

<style>
	.chip {
		display: inline-flex;
		align-items: center;
		padding: var(--space-1) var(--space-3);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-pill);
		background: var(--color-surface);
		color: var(--color-text-muted);
		font-size: var(--text-sm);
		text-decoration: none;
		white-space: nowrap;
		transition:
			background var(--transition-fast),
			color var(--transition-fast);
	}
	.chip:hover {
		background: var(--color-surface-raised);
		color: var(--color-text);
	}
	.active {
		border-color: var(--color-accent);
		color: var(--color-text);
	}
</style>
