<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { Spinner, Text } from '#lib/components/atoms/index.js';
	import { DefinitionToggleList, ErrorMessage, IndexerForm, type SiteState } from '#lib/components/molecules/index.js';
	import { queries } from '#lib/api/query.js';
	import { compactValues, initialValues, needsSettings } from '#lib/domain/settingsForm.js';
	import type { SetupDraft } from '#lib/domain/setup.js';
	import type { IndexerDefinition, IndexerInput } from '#lib/api/types.js';

	let { draft }: { draft: SetupDraft } = $props();
	const definitions = createQuery(() => queries.indexerDefinitions());

	/** The site whose settings form is open. */
	let editing = $state<string | null>(null);

	const states = $derived(Object.fromEntries(draft.indexers.map((ix): [string, SiteState] => [ix.definition ?? '', { enabled: true }])));
	const draftFor = (id: string) => draft.indexers.find((ix) => ix.definition === id);

	function put(input: IndexerInput) {
		const i = draft.indexers.findIndex((ix) => ix.definition === input.definition);
		if (i >= 0) draft.indexers[i] = input;
		else draft.indexers.push(input);
	}

	function onToggle(def: IndexerDefinition, on: boolean) {
		if (!on) {
			// Nothing is saved yet during setup, so switching off just drops it.
			draft.indexers = draft.indexers.filter((ix) => ix.definition !== def.id);
			if (editing === def.id) editing = null;
			return;
		}
		// A site that needs input (e.g. a sign-in) is switched on from its form.
		if (needsSettings(def)) editing = def.id;
		else put({ definition: def.id, settings: compactValues(initialValues(def)) });
	}

	function submit(input: IndexerInput) {
		put(input);
		editing = null;
	}

	export function validate(): Promise<boolean> {
		return Promise.resolve(true); // optional; checked on the server when finishing
	}
</script>

<Text>
	Optional: switch on the tracker sites to search. Sites that need a sign-in ask for it first. You can change them later in
	Settings.
</Text>
{#if definitions.isPending}
	<Spinner />
{:else if definitions.isError}
	<ErrorMessage error={definitions.error} />
{:else if definitions.data.length === 0}
	<Text size="sm" tone="muted">No site definitions are loaded yet. Upload some below.</Text>
{:else}
	<DefinitionToggleList
		definitions={definitions.data}
		{states}
		{editing}
		ontoggle={onToggle}
		onconfigure={(d) => (editing = editing === d.id ? null : d.id)}
	>
		{#snippet editor(def)}
			<IndexerForm definition={def} indexer={draftFor(def.id)} onsubmit={submit} oncancel={() => (editing = null)} />
		{/snippet}
	</DefinitionToggleList>
{/if}
