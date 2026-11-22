<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { Tone } from '#lib/status.js';

	interface Props {
		tone?: Tone;
		children: Snippet;
	}
	let { tone = 'neutral', children }: Props = $props();
</script>

<span class="badge {tone}">{@render children()}</span>

<style>
	.badge {
		display: inline-flex;
		align-items: center;
		padding: 0 var(--space-2);
		border-radius: var(--radius-pill);
		font-size: var(--text-xs);
		font-weight: var(--weight-medium);
		line-height: 1.6;
		white-space: nowrap;
		border: 1px solid currentColor;
	}
	.neutral {
		color: var(--color-neutral);
	}
	.info {
		color: var(--color-info);
	}
	.success {
		color: var(--color-success);
	}
	.warning {
		color: var(--color-warning);
	}
	.danger {
		color: var(--color-danger);
	}
</style>
