<script lang="ts">
	import Icon from './Icon.svelte';

	interface Props {
		/** Score out of 10. */
		value: number;
		/** How many people voted, when known. */
		votes?: number;
	}
	let { value, votes }: Props = $props();

	const label = $derived(`Rated ${value.toFixed(1)} out of 10${votes ? ` by ${votes.toLocaleString()} people` : ''}`);
</script>

<span class="rating" role="img" aria-label={label} title={label}>
	<Icon name="star" size={14} />
	<span aria-hidden="true">{value.toFixed(1)}</span>
</span>

<style>
	.rating {
		display: inline-flex;
		align-items: center;
		gap: var(--space-1);
		color: var(--color-warning);
		font-size: var(--text-sm);
		font-weight: var(--weight-bold);
		white-space: nowrap;
	}
</style>
