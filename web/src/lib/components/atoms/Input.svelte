<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';

	interface Props extends Omit<HTMLInputAttributes, 'value'> {
		value?: string | number;
		invalid?: boolean;
		monospace?: boolean;
	}
	let { value = $bindable(''), invalid = false, monospace = false, type = 'text', ...rest }: Props = $props();
</script>

<input {type} class:invalid class:monospace bind:value aria-invalid={invalid || undefined} {...rest} />

<style>
	input {
		width: 100%;
		padding: var(--space-2) var(--space-3);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-md);
		background: var(--color-bg);
		color: var(--color-text);
		font: inherit;
		font-size: var(--text-sm);
	}
	input:focus {
		border-color: var(--color-accent);
	}
	.invalid {
		border-color: var(--color-danger);
	}
	.monospace {
		font-family: var(--font-mono);
	}
</style>
