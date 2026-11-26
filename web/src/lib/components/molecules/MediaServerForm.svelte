<script lang="ts">
	import { untrack } from 'svelte';
	import { ApiError, fieldError, isFieldError } from '#lib/api/client.js';
	import { Button, Input, Select, Text } from '#lib/components/atoms/index.js';
	import { compactValues, initialValues, missingField } from '#lib/domain/settingsForm.js';
	import ErrorMessage from './ErrorMessage.svelte';
	import FormField from './FormField.svelte';
	import SettingsForm from './SettingsForm.svelte';
	import type { MediaServer, MediaServerDefinition, MediaServerInput } from '#lib/api/types.js';

	interface Props {
		/** The kinds of server that can be chosen; must not be empty. */
		definitions: MediaServerDefinition[];
		/** The saved server being edited, or undefined when adding one. */
		server?: MediaServer;
		busy?: boolean;
		error?: unknown;
		onsubmit: (input: MediaServerInput) => void;
		oncancel?: () => void;
	}
	let { definitions, server, busy = false, error, onsubmit, oncancel }: Props = $props();

	// Initial values are copied once; the form edits its own draft.
	const initial = untrack(() => server);
	const idPrefix = `ms-${initial?.id ?? 'new'}`;
	let definitionId = $state(untrack(() => initial?.definition ?? definitions[0].id));
	const definition = $derived(definitions.find((d) => d.id === definitionId) ?? definitions[0]);
	let name = $state(initial?.name ?? '');
	let values = $state(untrack(() => initialValues(definition, initial?.settings)));
	// Saved secrets only count for the saved server type.
	const secretsSet = $derived(definitionId === initial?.definition ? (initial?.secrets_set ?? []) : []);
	const missing = $derived(missingField(definition, values, secretsSet));

	function chooseType(id: string) {
		definitionId = id;
		const def = definitions.find((d) => d.id === id);
		if (def) values = initialValues(def, id === initial?.definition ? initial?.settings : undefined);
	}

	// Server errors about a setting are named "settings.<key>".
	const settingsError = $derived.by(() =>
		error instanceof ApiError && error.field?.startsWith('settings.')
			? { field: error.field.slice('settings.'.length), message: error.message }
			: undefined
	);

	function submit(e: SubmitEvent) {
		e.preventDefault();
		onsubmit({ name: name.trim() || undefined, definition: definitionId, settings: compactValues(values) });
	}
</script>

<form class="form" onsubmit={submit} aria-label={initial ? `${initial.name} settings` : 'New media server'}>
	<div class="grid">
		<FormField id="{idPrefix}-type" label="Type">
			{#snippet children(a)}
				<Select
					id={a.id}
					value={definitionId}
					options={definitions.map((d) => ({ value: d.id, label: d.name }))}
					onchange={(e) => chooseType(e.currentTarget.value)}
				/>
			{/snippet}
		</FormField>
		<FormField id="{idPrefix}-name" label="Name" error={fieldError(error, 'name')}>
			{#snippet children(a)}<Input {...a} bind:value={name} placeholder={definition.name} />{/snippet}
		</FormField>
	</div>
	{#if definition.description}<Text size="sm" tone="muted">{definition.description}</Text>{/if}
	{#key definitionId}
		<SettingsForm provider={definition} bind:values {secretsSet} error={settingsError} {idPrefix} />
	{/key}
	{#if error && !isFieldError(error, 'name') && !settingsError}
		<ErrorMessage {error} />
	{/if}
	<div class="actions">
		{#if missing}<Text size="sm" tone="muted" as="span">{missing.label} is required.</Text>{/if}
		{#if oncancel}<Button onclick={oncancel}>Cancel</Button>{/if}
		<Button type="submit" variant="primary" loading={busy} disabled={!!missing}>{initial ? 'Save' : 'Add'}</Button>
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
	.actions {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: flex-end;
		gap: var(--space-2);
	}
</style>
