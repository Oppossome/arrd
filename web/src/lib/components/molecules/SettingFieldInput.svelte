<script lang="ts">
	import { Input, Select, Text, Textarea } from '#lib/components/atoms/index.js';
	import type { SettingField, SettingOption } from '#lib/api/types.js';

	/** One integration setting, rendered from the field the server declares. */
	interface Props {
		field: SettingField;
		id: string;
		value: string | undefined;
		onchange: (value: string) => void;
		/** Choices for a select field with server-loaded options; fixed choices come from the field. */
		options?: SettingOption[];
		optionsLoading?: boolean;
		invalid?: boolean;
		describedBy?: string;
		/** A secret is saved on the server: blank keeps it. */
		secretSaved?: boolean;
	}
	let {
		field,
		id,
		value,
		onchange,
		options = [],
		optionsLoading = false,
		invalid = false,
		describedBy,
		secretSaved = false
	}: Props = $props();

	const placeholder = $derived(secretSaved ? 'Saved — leave blank to keep' : field.placeholder);
	const selectOptions = $derived([
		{ value: '', label: optionsLoading ? 'Loading…' : `Choose ${field.label.toLowerCase()}…` },
		...(field.choices ?? options).map((o) => ({ value: o.value, label: o.label }))
	]);

	async function load(e: Event) {
		const file = (e.currentTarget as HTMLInputElement).files?.[0];
		if (file) onchange(await file.text());
	}
</script>

{#if field.kind === 'textarea'}
	<div class="stack">
		<Textarea
			{id}
			value={value ?? ''}
			oninput={(e) => onchange(e.currentTarget.value)}
			{invalid}
			{placeholder}
			monospace
			rows={10}
			aria-describedby={describedBy}
		/>
		{#if field.file_accept}
			<label class="file">
				<Text size="xs" tone="muted" as="span">Or load a {field.file_accept} file:</Text>
				<input type="file" accept="{field.file_accept},text/plain" onchange={load} />
			</label>
		{/if}
	</div>
{:else if field.kind === 'select'}
	<Select
		{id}
		value={value ?? ''}
		onchange={(e) => onchange(e.currentTarget.value)}
		options={selectOptions}
		disabled={optionsLoading}
		aria-describedby={describedBy}
		aria-invalid={invalid}
	/>
{:else}
	<Input
		{id}
		value={value ?? ''}
		oninput={(e) => onchange(e.currentTarget.value)}
		{invalid}
		{placeholder}
		type={field.kind === 'password' ? 'password' : 'text'}
		autocomplete={field.secret ? 'new-password' : 'off'}
		aria-describedby={describedBy}
	/>
{/if}

<style>
	.stack {
		display: grid;
		gap: var(--space-2);
	}
	.file {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		font-size: var(--text-xs);
	}
</style>
