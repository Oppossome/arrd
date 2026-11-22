<script lang="ts">
	import type { HTMLTextareaAttributes } from 'svelte/elements';

	interface Props extends Omit<HTMLTextareaAttributes, 'value'> {
		value?: string;
		invalid?: boolean;
		monospace?: boolean;
	}
	let { value = $bindable(''), invalid = false, monospace = false, rows = 6, ...rest }: Props = $props();
</script>

<textarea class:invalid class:monospace {rows} bind:value aria-invalid={invalid || undefined} {...rest}></textarea>

<style>
	textarea {
		width: 100%;
		padding: var(--space-2) var(--space-3);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-md);
		background: var(--color-bg);
		color: var(--color-text);
		font: inherit;
		font-size: var(--text-sm);
		resize: vertical;
	}
	.invalid {
		border-color: var(--color-danger);
	}
	.monospace {
		font-family: var(--font-mono);
	}
</style>
