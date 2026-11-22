<script lang="ts">
	import type { Snippet } from 'svelte';
	interface Props {
		tone?: 'default' | 'muted' | 'danger' | 'success' | 'warning';
		size?: 'xs' | 'sm' | 'md';
		as?: 'p' | 'span';
		mono?: boolean;
		id?: string;
		children: Snippet;
	}
	let { tone = 'default', size = 'md', as = 'p', mono = false, id, children }: Props = $props();
</script>

<svelte:element this={as} {id} class="text {tone} {size}" class:mono>{@render children()}</svelte:element>

<style>
	.text {
		margin: 0;
	}
	.xs {
		font-size: var(--text-xs);
	}
	.sm {
		font-size: var(--text-sm);
	}
	.md {
		font-size: var(--text-md);
	}
	.muted {
		color: var(--color-text-muted);
	}
	.danger {
		color: var(--color-danger);
	}
	.success {
		color: var(--color-success);
	}
	.warning {
		color: var(--color-warning);
	}
	.mono {
		font-family: var(--font-mono);
		word-break: break-all;
	}
</style>
