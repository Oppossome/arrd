<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';

	interface Props extends Omit<HTMLInputAttributes, 'type' | 'checked'> {
		checked?: boolean;
		label: string;
	}
	let { checked = $bindable(false), label, ...rest }: Props = $props();
</script>

<label class="checkbox">
	<input type="checkbox" bind:checked {...rest} />
	<span>{label}</span>
</label>

<style>
	.checkbox {
		display: inline-flex;
		align-items: center;
		gap: var(--space-2);
		font-size: var(--text-sm);
		cursor: pointer;
	}
	input {
		accent-color: var(--color-accent-strong);
	}
</style>
