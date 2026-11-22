<script lang="ts" module>
	/** What the list shows for a site that has an indexer (or a draft of one). */
	export interface SiteState {
		enabled: boolean;
		/** The site has been tested successfully. */
		tested?: boolean;
		/** The last error, ready to show (e.g. "Last error 5m ago: …"). */
		error?: string;
	}
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Badge, Button, Checkbox, IconButton, Text, Toggle } from '#lib/components/atoms/index.js';
	import type { IndexerDefinition } from '#lib/types.js';
	import SearchBox from './SearchBox.svelte';

	interface Props {
		definitions: IndexerDefinition[];
		/** By definition id; sites without an entry have never been switched on. */
		states: Record<string, SiteState>;
		/** The site whose toggle is being saved. */
		busy?: string | null;
		/** The site being tested. */
		testing?: string | null;
		/** The site whose editor is open below its row. */
		editing?: string | null;
		editor?: Snippet<[IndexerDefinition]>;
		ontoggle: (def: IndexerDefinition, on: boolean) => void;
		onconfigure?: (def: IndexerDefinition) => void;
		ontest?: (def: IndexerDefinition) => void;
		ondelete?: (def: IndexerDefinition) => void;
	}
	let { definitions, states, busy = null, testing = null, editing = null, editor, ontoggle, onconfigure, ontest, ondelete }: Props =
		$props();

	let filter = $state('');
	let onlyOn = $state(false);

	const onCount = $derived(definitions.filter((d) => states[d.id]?.enabled).length);
	const shown = $derived.by(() => {
		const q = filter.trim().toLowerCase();
		return definitions.filter(
			(d) =>
				(!onlyOn || states[d.id]?.enabled || d.id === editing) &&
				(!q || d.name.toLowerCase().includes(q) || d.id.toLowerCase().includes(q))
		);
	});
</script>

<div class="wrap">
	<div class="tools">
		<SearchBox bind:value={filter} label="Filter sites" placeholder="Filter by name or id" />
		<Checkbox bind:checked={onlyOn} label="Only switched on ({onCount})" />
	</div>
	{#if shown.length === 0}
		<Text size="sm" tone="muted">No sites match.</Text>
	{:else}
		<ul class="list">
			{#each shown as d (d.id)}
				{@const state = states[d.id]}
				<li class="item" class:open={d.id === editing}>
					<div class="row">
						<Toggle
							checked={!!state?.enabled}
							label="{state?.enabled ? 'Switch off' : 'Switch on'} {d.name}"
							disabled={busy === d.id || (!!d.unsupported && !state?.enabled)}
							onchange={(on) => ontoggle(d, on)}
						/>
						<div class="info">
							<div class="name">
								<Text as="span" size="sm">{d.name}</Text>
								<Text as="span" size="xs" tone="muted" mono>{d.id}</Text>
								{#if d.unsupported}<Badge tone="warning">Unsupported</Badge>{/if}
								{#if state?.tested}<Badge tone="success">Tested</Badge>{/if}
							</div>
							{#if d.description}<Text size="xs" tone="muted">{d.description}</Text>{/if}
							{#if d.unsupported}<Text size="xs" tone="muted">{d.unsupported}</Text>{/if}
							{#if state?.error}<Text size="xs" tone="danger">{state.error}</Text>{/if}
						</div>
						<div class="actions">
							{#if ontest && state?.enabled}
								<Button size="sm" loading={testing === d.id} onclick={() => ontest(d)}>Test</Button>
							{/if}
							{#if onconfigure && state && !d.unsupported}
								<IconButton icon="settings" label="Settings for {d.name}" onclick={() => onconfigure(d)} />
							{/if}
							{#if ondelete}
								<IconButton icon="trash" tone="danger" label="Delete {d.name}" onclick={() => ondelete(d)} />
							{/if}
						</div>
					</div>
					{#if d.id === editing && editor}
						<div class="editor">{@render editor(d)}</div>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}
</div>

<style>
	.wrap {
		display: grid;
		gap: var(--space-3);
	}
	.tools {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-3) var(--space-5);
	}
	.list {
		display: grid;
		max-height: 36rem;
		overflow-y: auto;
		margin: 0;
		padding: 0;
		list-style: none;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-md);
		background: var(--color-surface);
	}
	.item + .item {
		border-top: 1px solid var(--color-border);
	}
	.row {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-2) var(--space-3);
	}
	.info {
		display: grid;
		flex: 1;
		gap: var(--space-1);
		min-width: 0;
	}
	.name,
	.actions {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-2);
	}
	.editor {
		padding: var(--space-3) var(--space-4) var(--space-4);
		border-top: 1px dashed var(--color-border);
	}
	.open {
		background: var(--color-surface-raised);
	}
</style>
