<script lang="ts" generics="V extends string | number">
	import type { HTMLSelectAttributes } from 'svelte/elements';

	interface Props extends Omit<HTMLSelectAttributes, 'value'> {
		value: V;
		options: { value: V; label: string }[];
	}
	let { value = $bindable(), options, ...rest }: Props = $props();
</script>

<select bind:value {...rest}>
	{#each options as opt (opt.value)}
		<option value={opt.value}>{opt.label}</option>
	{/each}
</select>

<style>
	select {
		width: 100%;
		padding: var(--space-2) var(--space-3);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-md);
		background: var(--color-bg);
		color: var(--color-text);
		font: inherit;
		font-size: var(--text-sm);
	}
</style>
