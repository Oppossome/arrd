<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { Badge, IconButton, Spinner, Text } from '#lib/components/atoms/index.js';
	import { ConfirmDialog, DefinitionToggleList, EmptyState, ErrorMessage, IndexerForm, type SiteState } from '#lib/components/molecules/index.js';
	import { timeAgo } from '#lib/format.js';
	import { queries, useMutation } from '#lib/query.js';
	import { compactValues, initialValues, needsSettings } from '#lib/settingsForm.js';
	import { toasts } from '#lib/toast.svelte.js';
	import type { Indexer, IndexerDefinition, IndexerInput } from '#lib/types.js';

	const indexers = createQuery(() => queries.indexers());
	const definitions = createQuery(() => queries.indexerDefinitions());
	const vpn = createQuery(() => queries.vpnSettings(false));
	const save = useMutation('saveIndexer');
	const toggle = useMutation('saveIndexer');
	const del = useMutation('deleteIndexer');
	const delDefinition = useMutation('deleteIndexerDefinition');
	const test = useMutation('testIndexer');

	/** Each site's indexer; a site has at most one. */
	const byDefinition = $derived(new Map((indexers.data ?? []).map((ix) => [ix.definition, ix])));
	/** Indexers whose site definition is no longer loaded. */
	const orphans = $derived.by(() => {
		const known = new Set((definitions.data ?? []).map((d) => d.id));
		return (indexers.data ?? []).filter((ix) => !known.has(ix.definition));
	});
	const states = $derived.by(() => {
		const out: Record<string, SiteState> = {};
		for (const [id, ix] of byDefinition) {
			out[id] = {
				enabled: ix.enabled,
				tested: !!ix.caps,
				error: ix.last_error ? `Last error ${timeAgo(ix.last_error_at)}: ${ix.last_error}` : undefined
			};
		}
		return out;
	});

	/** The site whose settings form is open. */
	let editing = $state<string | null>(null);
	let toggling = $state<string | null>(null);
	let testing = $state<string | null>(null);
	let deletingDefinition = $state<IndexerDefinition | null>(null);
	let deletingIndexer = $state<Indexer | null>(null);

	function setEnabled(def: IndexerDefinition, ix: Indexer, enabled: boolean) {
		toggling = def.id;
		toggle.mutate(
			{ id: ix.id, input: { enabled } },
			{
				onSuccess: () => toasts.push('success', `${def.name} switched ${enabled ? 'on' : 'off'}`),
				onSettled: () => (toggling = null)
			}
		);
	}

	function onToggle(def: IndexerDefinition, on: boolean) {
		toggle.reset();
		const ix = byDefinition.get(def.id);
		if (ix) return setEnabled(def, ix, on);
		if (!on) return;
		// A site that needs input (e.g. a sign-in) is switched on from its form.
		if (needsSettings(def)) {
			save.reset();
			editing = def.id;
			return;
		}
		toggling = def.id;
		toggle.mutate(
			{ input: { definition: def.id, settings: compactValues(initialValues(def)), enabled: true } },
			{
				onSuccess: () => toasts.push('success', `${def.name} switched on`),
				onSettled: () => (toggling = null)
			}
		);
	}

	function configure(def: IndexerDefinition) {
		save.reset();
		editing = editing === def.id ? null : def.id;
	}

	function submit(def: IndexerDefinition, input: IndexerInput) {
		const ix = byDefinition.get(def.id);
		save.mutate(ix ? { id: ix.id, input } : { input: { ...input, enabled: true } }, {
			onSuccess: () => {
				editing = null;
				toasts.push('success', ix ? `${def.name} saved` : `${def.name} switched on`);
			}
		});
	}

	function runTest(def: IndexerDefinition) {
		const ix = byDefinition.get(def.id);
		if (!ix) return;
		testing = def.id;
		test.mutate(ix.id, {
			onSuccess: () => toasts.push('success', `${def.name} works`),
			onSettled: () => (testing = null)
		});
	}
</script>

<section class="panel" aria-label="Indexers">
	{#if vpn.data && !vpn.data.configured}
		<Text size="sm" tone="muted">Indexers use your normal connection. Set up a VPN to route them through it.</Text>
	{/if}

	{#if orphans.length}
		<div class="orphans">
			<Text size="sm" tone="warning">These indexers' site definitions are no longer loaded. Upload them again, or delete the indexers.</Text>
			<ul class="list">
				{#each orphans as ix (ix.id)}
					<li class="orphan">
						<Text as="span" size="sm">{ix.name}</Text>
						<Text as="span" size="xs" tone="muted" mono>{ix.definition}</Text>
						{#if !ix.enabled}<Badge>Off</Badge>{/if}
						<span class="spacer"></span>
						<IconButton icon="trash" tone="danger" label="Delete {ix.name}" onclick={() => (deletingIndexer = ix)} />
					</li>
				{/each}
			</ul>
		</div>
	{/if}

	{#if definitions.isPending || indexers.isPending}
		<Spinner />
	{:else if definitions.isError}
		<ErrorMessage error={definitions.error} />
	{:else if indexers.isError}
		<ErrorMessage error={indexers.error} />
	{:else if definitions.data.length === 0}
		<EmptyState message="No site definitions are loaded yet. Upload some to choose which sites to search." />
	{:else}
		{#if toggle.error}<ErrorMessage error={toggle.error} />{/if}
		<DefinitionToggleList
			definitions={definitions.data}
			{states}
			busy={toggling}
			{testing}
			{editing}
			ontoggle={onToggle}
			onconfigure={configure}
			ontest={runTest}
			ondelete={(d) => (deletingDefinition = d)}
		>
			{#snippet editor(def)}
				<IndexerForm
					definition={def}
					indexer={byDefinition.get(def.id)}
					busy={save.isPending}
					error={save.error}
					onsubmit={(input) => submit(def, input)}
					oncancel={() => (editing = null)}
				/>
			{/snippet}
		</DefinitionToggleList>
	{/if}
</section>

<ConfirmDialog
	open={deletingDefinition !== null}
	title="Delete the {deletingDefinition?.name} definition?"
	confirmLabel="Delete"
	danger
	busy={delDefinition.isPending}
	onconfirm={() => deletingDefinition && delDefinition.mutate(deletingDefinition.id, { onSuccess: () => (deletingDefinition = null) })}
	oncancel={() => (deletingDefinition = null)}
>
	<Text size="sm">
		{#if deletingDefinition && byDefinition.has(deletingDefinition.id)}
			Its indexer keeps its settings but stops working until the definition is uploaded again.
		{:else}
			The site can be added again by uploading its definition.
		{/if}
	</Text>
	{#if delDefinition.error}<ErrorMessage error={delDefinition.error} />{/if}
</ConfirmDialog>

<ConfirmDialog
	open={deletingIndexer !== null}
	title="Delete {deletingIndexer?.name}?"
	confirmLabel="Delete"
	danger
	busy={del.isPending}
	onconfirm={() => deletingIndexer && del.mutate(deletingIndexer.id, { onSuccess: () => (deletingIndexer = null) })}
	oncancel={() => (deletingIndexer = null)}
>
	<Text size="sm">Its saved settings are removed.</Text>
</ConfirmDialog>

<style>
	.panel {
		display: grid;
		gap: var(--space-4);
	}
	.orphans {
		display: grid;
		gap: var(--space-2);
	}
	.list {
		display: grid;
		gap: var(--space-1);
		margin: 0;
		padding: 0;
		list-style: none;
	}
	.orphan {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-1) var(--space-2);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-md);
	}
	.spacer {
		flex: 1;
	}
</style>
