<script lang="ts">
	import type { HTMLButtonAttributes } from 'svelte/elements';
	import Icon, { type IconName } from './Icon.svelte';

	interface Props extends HTMLButtonAttributes {
		icon: IconName;
		label: string;
		tone?: 'default' | 'danger';
		/** Spin the icon (e.g. a refresh in progress); the button is disabled meanwhile. */
		spinning?: boolean;
	}
	let { icon, label, tone = 'default', spinning = false, type = 'button', disabled, ...rest }: Props = $props();
</script>

<button
	{type}
	class="icon-btn {tone}"
	class:spinning
	aria-label={label}
	title={label}
	aria-busy={spinning || undefined}
	disabled={disabled || spinning}
	{...rest}
>
	<Icon name={icon} />
</button>

<style>
	.icon-btn {
		display: inline-grid;
		place-items: center;
		padding: var(--space-2);
		border: 1px solid transparent;
		border-radius: var(--radius-md);
		background: transparent;
		color: var(--color-text-muted);
		cursor: pointer;
	}
	.icon-btn:hover:not(:disabled) {
		color: var(--color-text);
		background: var(--color-surface-raised);
	}
	.danger:hover:not(:disabled) {
		color: var(--color-danger);
	}
	.icon-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.spinning:disabled {
		opacity: 1;
		cursor: progress;
	}
	.spinning :global(svg) {
		animation: spin 900ms linear infinite;
	}
	@media (prefers-reduced-motion: reduce) {
		.spinning :global(svg) {
			animation-duration: 3s;
		}
	}
	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
