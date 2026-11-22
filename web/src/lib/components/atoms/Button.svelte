<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { HTMLButtonAttributes } from 'svelte/elements';

	interface Props extends HTMLButtonAttributes {
		variant?: 'primary' | 'secondary' | 'danger' | 'ghost';
		size?: 'sm' | 'md';
		loading?: boolean;
		children: Snippet;
	}

	let { variant = 'secondary', size = 'md', loading = false, type = 'button', disabled, children, ...rest }: Props =
		$props();
</script>

<button {type} class="btn {variant} {size}" disabled={disabled || loading} aria-busy={loading} {...rest}>
	{#if loading}<span class="spin" aria-hidden="true"></span>{/if}
	{@render children()}
</button>

<style>
	.btn {
		display: inline-flex;
		align-items: center;
		gap: var(--space-2);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-md);
		background: var(--color-surface-raised);
		color: var(--color-text);
		font: inherit;
		font-weight: var(--weight-medium);
		cursor: pointer;
		transition: background var(--transition-fast), border-color var(--transition-fast);
	}
	.md {
		padding: var(--space-2) var(--space-4);
		font-size: var(--text-sm);
	}
	.sm {
		padding: var(--space-1) var(--space-3);
		font-size: var(--text-xs);
	}
	.btn:hover:not(:disabled) {
		border-color: var(--color-text-muted);
	}
	.primary {
		background: var(--color-accent-strong);
		border-color: var(--color-accent-strong);
		color: var(--color-on-accent);
	}
	.primary:hover:not(:disabled) {
		background: var(--color-accent);
		border-color: var(--color-accent);
	}
	.danger {
		color: var(--color-danger);
		border-color: var(--color-danger);
		background: transparent;
	}
	.ghost {
		background: transparent;
		border-color: transparent;
	}
	.btn:disabled {
		opacity: 0.55;
		cursor: not-allowed;
	}
	.spin {
		width: 0.9em;
		height: 0.9em;
		border: 2px solid currentColor;
		border-right-color: transparent;
		border-radius: var(--radius-pill);
		animation: rot 0.8s linear infinite;
	}
	@keyframes rot {
		to {
			transform: rotate(360deg);
		}
	}
</style>
