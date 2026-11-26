<script lang="ts" generics="V extends string | number">
	import { Icon } from '#lib/components/atoms/index.js';
	import Dropdown from './Dropdown.svelte';

	interface Props {
		/** What is being chosen, e.g. "Sort"; names the trigger and the panel. */
		label: string;
		value: V;
		options: { value: V; label: string }[];
	}
	let { label, value = $bindable(), options }: Props = $props();

	const current = $derived(options.find((o) => o.value === value));
</script>

<!-- A single choice: picking one closes the menu. The trigger shows it. -->
<Dropdown {label}>
	{#snippet button()}
		<span class="visually-hidden">{label}:</span>
		{current?.label ?? label}
	{/snippet}

	{#snippet children(close: () => void)}
		<ul>
			{#each options as opt (opt.value)}
				<li>
					<button
						type="button"
						aria-pressed={opt.value === value}
						onclick={() => {
							value = opt.value;
							close();
						}}
					>
						<span class="mark" aria-hidden="true">
							{#if opt.value === value}<Icon name="check" size={14} />{/if}
						</span>
						{opt.label}
					</button>
				</li>
			{/each}
		</ul>
	{/snippet}
</Dropdown>

<style>
	ul {
		margin: 0;
		padding: var(--space-1) 0;
		list-style: none;
	}
	button {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		width: 100%;
		padding: var(--space-2) var(--space-3);
		border: 0;
		background: transparent;
		color: inherit;
		font: inherit;
		text-align: left;
		white-space: nowrap;
		cursor: pointer;
	}
	button:hover {
		background: var(--color-surface);
	}
	.mark {
		display: inline-flex;
		width: 14px;
		color: var(--color-accent);
	}
</style>
