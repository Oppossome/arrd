<script lang="ts">
	import { Text, Toggle } from '#lib/components/atoms/index.js';
	import type { SettingOption, SettingValues, SettingsSchema } from '#lib/api/types.js';
	import FormField from './FormField.svelte';
	import SettingFieldInput from './SettingFieldInput.svelte';

	/**
	 * The settings form for one integration, generated from the fields it
	 * declares. Option lists are loaded by the caller.
	 */
	interface Props {
		/** What declares the fields: a VPN provider, an indexer site. */
		provider: SettingsSchema;
		values: SettingValues;
		/** Loaded choices by field key. */
		options?: Record<string, SettingOption[] | undefined>;
		/** Field keys whose choices are still loading. */
		loading?: readonly string[];
		/** Secret fields with a saved value. */
		secretsSet?: readonly string[];
		/** A server error about one field. */
		error?: { field: string; message: string };
		idPrefix?: string;
	}
	let {
		provider,
		values = $bindable(),
		options = {},
		loading = [],
		secretsSet = [],
		error,
		idPrefix = 'vpn'
	}: Props = $props();
</script>

<div class="fields">
	{#each provider.fields as field (field.key)}
		{@const id = `${idPrefix}-${field.key}`}
		{@const fieldError = error?.field === field.key ? error.message : undefined}
		{#if field.kind === 'toggle'}
			<div class="toggle">
				<div class="toggle-row">
					<Toggle
						label={field.label}
						checked={values[field.key] === 'true'}
						onchange={(on) => (values[field.key] = on ? 'true' : 'false')}
					/>
					<!-- The switch already carries this name for assistive tech. -->
					<span class="toggle-label" aria-hidden="true">{field.label}</span>
				</div>
				{#if field.help}<Text size="xs" tone="muted">{field.help}</Text>{/if}
				{#if fieldError}<Text size="xs" tone="danger">{fieldError}</Text>{/if}
			</div>
		{:else}
			<FormField {id} label={field.required ? field.label : `${field.label} (optional)`} hint={field.help} error={fieldError}>
				{#snippet children(a)}
					<SettingFieldInput
						{field}
						id={a.id}
						value={values[field.key]}
						onchange={(v) => (values[field.key] = v)}
						options={options[field.key]}
						optionsLoading={loading.includes(field.key)}
						invalid={a.invalid}
						describedBy={a['aria-describedby']}
						secretSaved={!!field.secret && secretsSet.includes(field.key)}
					/>
				{/snippet}
			</FormField>
		{/if}
	{/each}
</div>

<style>
	.fields {
		display: grid;
		gap: var(--space-4);
	}
	.toggle {
		display: grid;
		gap: var(--space-1);
	}
	.toggle-row {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}
	.toggle-label {
		font-size: var(--text-sm);
		font-weight: 500;
	}
</style>
