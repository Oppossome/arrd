<script lang="ts">
	import { CountryFlag, Icon } from '#lib/components/atoms/index.js';
	import type { CountryCount } from '#lib/types.js';

	interface Props {
		countries: CountryCount[];
		/** How many countries to name before "+N". */
		limit?: number;
		onclick: () => void;
	}
	let { countries, limit = 3, onclick }: Props = $props();

	const total = $derived(countries.reduce((n, c) => n + c.count, 0));
	const shown = $derived(countries.slice(0, limit));
	const others = $derived(countries.slice(limit).reduce((n, c) => n + c.count, 0));
</script>

<button type="button" class="summary" {onclick} aria-label="Show {total} peer{total === 1 ? '' : 's'}">
	<Icon name="users" size={14} />
	{#if total === 0}
		<span class="muted">No peers</span>
	{:else}
		{#each shown as c (c.country)}
			<span class="entry"><CountryFlag code={c.country} />{c.count}</span>
		{/each}
		{#if others > 0}<span class="muted">+{others}</span>{/if}
	{/if}
</button>

<style>
	.summary {
		display: inline-flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-1) var(--space-2);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-pill);
		background: var(--color-overlay);
		color: var(--color-text);
		font: inherit;
		font-size: var(--text-xs);
		cursor: pointer;
	}
	.summary:hover {
		border-color: var(--color-text-muted);
	}
	.entry {
		display: inline-flex;
		align-items: center;
		gap: var(--space-1);
	}
	.muted {
		color: var(--color-text-muted);
	}
</style>
