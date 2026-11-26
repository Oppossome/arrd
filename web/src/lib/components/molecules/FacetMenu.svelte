<script lang="ts">
	import Dropdown from './Dropdown.svelte';
	import type { Facet } from '#lib/domain/releases.js';

	interface Props {
		/** The field the menu filters by, e.g. "Quality". */
		label: string;
		/** The values to choose from, with how many results have each. */
		facets: Facet[];
		/** The values chosen. */
		selected: string[];
		/** The user checked or unchecked value. */
		ontoggle: (value: string) => void;
		/** Clears every chosen value; without it the menu offers no Clear. */
		onclear?: () => void;
	}
	let { label, facets, selected, ontoggle, onclear }: Props = $props();
</script>

<!-- Checking values leaves the menu open, so several can be picked. -->
<Dropdown {label} active={selected.length > 0}>
	{#snippet button()}
		{label}
		{#if selected.length > 0}<span class="count">({selected.length})</span>{/if}
	{/snippet}

	<ul>
		{#each facets as f (f.value)}
			<li>
				<label>
					<input type="checkbox" checked={selected.includes(f.value)} onchange={() => ontoggle(f.value)} />
					<span class="value">{f.value}</span>
					<span class="count">{f.count}</span>
				</label>
			</li>
		{/each}
	</ul>
	{#if onclear && selected.length > 0}
		<button type="button" class="clear" onclick={onclear}>Clear</button>
	{/if}
</Dropdown>

<style>
	.count {
		color: var(--color-text-muted);
		font-variant-numeric: tabular-nums;
	}
	ul {
		min-width: 14rem;
		max-height: 18rem;
		margin: 0;
		padding: var(--space-1) 0;
		overflow-y: auto;
		list-style: none;
	}
	label {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-3);
		cursor: pointer;
	}
	label:hover {
		background: var(--color-surface);
	}
	input {
		margin: 0;
		accent-color: var(--color-accent-strong);
	}
	.value {
		flex: 1;
		white-space: nowrap;
	}
	.clear {
		display: block;
		width: 100%;
		padding: var(--space-2) var(--space-3);
		border: 0;
		border-top: 1px solid var(--color-border);
		background: transparent;
		color: var(--color-text-muted);
		font: inherit;
		font-size: var(--text-xs);
		text-align: left;
		cursor: pointer;
	}
	.clear:hover {
		color: var(--color-text);
	}
</style>
