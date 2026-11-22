<script lang="ts">
	import type { HTMLButtonAttributes } from 'svelte/elements';
	import Icon, { type IconName } from './Icon.svelte';

	interface Props extends HTMLButtonAttributes {
		icon: IconName;
		label: string;
		tone?: 'default' | 'danger';
	}
	let { icon, label, tone = 'default', type = 'button', ...rest }: Props = $props();
</script>

<button {type} class="icon-btn {tone}" aria-label={label} title={label} {...rest}>
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
</style>
