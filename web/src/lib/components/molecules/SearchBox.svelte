<script lang="ts">
	import { Icon, Input } from '#lib/components/atoms/index.js';

	interface Props {
		value: string;
		label: string;
		placeholder?: string;
	}
	let { value = $bindable(''), label, placeholder }: Props = $props();
</script>

<div class="search">
	<Icon name="search" />
	<Input type="search" bind:value aria-label={label} {placeholder} />
</div>

<style>
	.search {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		color: var(--color-text-muted);
	}
</style>
