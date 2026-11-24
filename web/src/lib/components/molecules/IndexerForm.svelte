<script lang="ts">
	import { untrack } from 'svelte';
	import { ApiError, fieldError, isFieldError } from '#lib/api/client.js';
	import { Button, Checkbox, Input, Text } from '#lib/components/atoms/index.js';
	import { compactValues, initialValues, missingField } from '#lib/domain/settingsForm.js';
	import ErrorMessage from './ErrorMessage.svelte';
	import FormField from './FormField.svelte';
	import SettingsForm from './SettingsForm.svelte';
	import type { IndexerDefinition, IndexerInput } from '#lib/api/types.js';

	interface Props {
		/** The site the settings are for. */
		definition: IndexerDefinition;
		/** The saved indexer (or setup draft) for the site, or undefined when switching it on for the first time. */
		indexer?: IndexerInput & { secrets_set?: string[] };
		busy?: boolean;
		error?: unknown;
		onsubmit: (input: IndexerInput) => void;
		oncancel?: () => void;
	}
	let { definition, indexer, busy = false, error, onsubmit, oncancel }: Props = $props();

	const cats = (s: string) =>
		s
			.split(',')
			.map((x) => Number(x.trim()))
			.filter((n) => Number.isInteger(n) && n > 0);

	// Initial values are copied once; the form edits its own draft.
	const initial = untrack(() => indexer);
	const site = untrack(() => definition);
	let values = $state(initialValues(site, initial?.settings));
	const secretsSet = initial?.secrets_set ?? [];
	const missing = $derived(missingField(definition, values, secretsSet));

	let movieCats = $state((initial?.movie_categories ?? [2000]).join(', '));
	let tvCats = $state((initial?.tv_categories ?? [5000]).join(', '));
	let rss = $state(initial?.enable_rss ?? true);
	let search = $state(initial?.enable_search ?? true);
	let priority = $state(initial?.priority ?? 25);
	let minSeeders = $state(initial?.min_seeders ?? 1);

	const fieldErr = (f: string) => fieldError(error, f);
	// Server errors about a setting are named "settings.<key>".
	const settingsError = $derived.by(() =>
		error instanceof ApiError && error.field?.startsWith('settings.')
			? { field: error.field.slice('settings.'.length), message: error.message }
			: undefined
	);

	function submit(e: SubmitEvent) {
		e.preventDefault();
		onsubmit({
			definition: definition.id,
			settings: compactValues(values),
			movie_categories: cats(movieCats),
			tv_categories: cats(tvCats),
			// Sites that cannot list recent releases have RSS off.
			enable_rss: definition.rss ? rss : false,
			enable_search: search,
			priority: Number(priority),
			min_seeders: Number(minSeeders)
		});
	}
</script>

<form class="form" onsubmit={submit} aria-label="{definition.name} settings">
	{#if definition.unsupported}<Text size="sm" tone="danger">This site cannot be used: {definition.unsupported}</Text>{/if}
	<SettingsForm provider={definition} bind:values {secretsSet} error={settingsError} idPrefix="ix-{definition.id}" />
	<div class="grid">
		<FormField id="ix-{definition.id}-mc" label="Movie categories">
			{#snippet children(a)}<Input {...a} bind:value={movieCats} />{/snippet}
		</FormField>
		<FormField id="ix-{definition.id}-tc" label="TV categories">
			{#snippet children(a)}<Input {...a} bind:value={tvCats} />{/snippet}
		</FormField>
		<FormField id="ix-{definition.id}-prio" label="Priority" hint="Lower is preferred">
			{#snippet children(a)}<Input {...a} bind:value={priority} type="number" />{/snippet}
		</FormField>
		<FormField id="ix-{definition.id}-seed" label="Minimum seeders" error={fieldErr('min_seeders')}>
			{#snippet children(a)}<Input {...a} bind:value={minSeeders} type="number" min="0" />{/snippet}
		</FormField>
	</div>
	<div class="checks">
		<Checkbox bind:checked={search} label="Use for searches" />
		<Checkbox
			checked={definition.rss ? rss : false}
			onchange={(e) => (rss = e.currentTarget.checked)}
			disabled={!definition.rss}
			label={definition.rss ? 'Use for RSS' : 'Use for RSS (this site cannot list recent releases)'}
		/>
	</div>
	{#if error && !isFieldError(error, 'min_seeders') && !settingsError}
		<ErrorMessage {error} />
	{/if}
	<div class="actions">
		{#if missing}<Text size="sm" tone="muted" as="span">{missing.label} is required.</Text>{/if}
		{#if oncancel}<Button onclick={oncancel}>Cancel</Button>{/if}
		<Button type="submit" variant="primary" loading={busy} disabled={!!definition.unsupported || !!missing}>
			{indexer ? 'Save' : 'Switch on'}
		</Button>
	</div>
</form>

<style>
	.form {
		display: grid;
		gap: var(--space-4);
	}
	.grid {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
		gap: var(--space-3);
	}
	.checks,
	.actions {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-4);
	}
	.actions {
		justify-content: flex-end;
		gap: var(--space-2);
	}
</style>
