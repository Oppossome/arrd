<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { Button, Select, Spinner, Text } from '#lib/components/atoms/index.js';
	import { ErrorMessage, FormField, SettingsForm } from '#lib/components/molecules/index.js';
	import { ApiError } from '#lib/api.js';
	import { queries, useMutation, useVPNOptions } from '#lib/query.js';
	import { vpnProblem, type SetupDraft } from '#lib/setup.js';
	import { initialValues, missingField, toVPNInput } from '#lib/settingsForm.js';

	let { draft }: { draft: SetupDraft } = $props();
	const providers = createQuery(() => queries.vpnProviders());
	const test = useMutation('validateVPN');
	let problem = $state<string | null>(null);
	let tested = $state('');

	const provider = $derived(providers.data?.find((p) => p.id === draft.vpnProvider));
	const options = useVPNOptions(() => provider);
	const missing = $derived(provider ? missingField(provider, draft.vpnValues) : null);
	const current = $derived(JSON.stringify(toVPNInput(draft.vpnProvider, draft.vpnValues)));
	const formError = $derived(
		test.error instanceof ApiError && test.error.field ? { field: test.error.field, message: test.error.message } : undefined
	);

	// Changed settings need a new test.
	$effect(() => {
		if (current !== tested) draft.vpnTestedIP = '';
	});

	function choose(id: string) {
		draft.vpnProvider = id;
		const p = providers.data?.find((d) => d.id === id);
		draft.vpnValues = p ? initialValues(p) : {};
		test.reset();
	}

	function runTest() {
		const snapshot = current;
		test.mutate(toVPNInput(draft.vpnProvider, draft.vpnValues), {
			onSuccess: (r) => {
				tested = snapshot;
				draft.vpnTestedIP = r.public_ip;
				problem = null;
			}
		});
	}

	export function validate(): Promise<boolean> {
		problem = vpnProblem(draft);
		return Promise.resolve(!problem);
	}
</script>

<Text>
	Optional but recommended: with a VPN, all indexer searches and downloads go through it, and they stop rather than use
	your normal connection if the tunnel is down. Without one, they use your normal connection.
</Text>
{#if providers.isPending}
	<Spinner />
{:else if providers.isError}
	<ErrorMessage error={providers.error} />
{:else}
	<FormField id="setup-vpn-provider" label="VPN">
		{#snippet children(a)}
			<Select
				id={a.id}
				value={draft.vpnProvider}
				options={[{ value: '', label: 'No VPN' }, ...providers.data.map((p) => ({ value: p.id, label: p.name }))]}
				onchange={(e) => choose(e.currentTarget.value)}
			/>
		{/snippet}
	</FormField>
	{#if provider}
		{#if provider.description}<Text size="sm" tone="muted">{provider.description}</Text>{/if}
		<SettingsForm
			{provider}
			bind:values={draft.vpnValues}
			options={options.options}
			loading={options.loading}
			error={formError}
			idPrefix="setup-vpn"
		/>
		{#if options.error}<ErrorMessage error={options.error} />{/if}
		<div class="row">
			<Button disabled={!!missing} loading={test.isPending} onclick={runTest}>Test connection</Button>
			{#if draft.vpnTestedIP}<Text tone="success" size="sm" as="span">Connected. Traffic leaves from {draft.vpnTestedIP}.</Text>{/if}
			{#if missing}<Text size="sm" tone="muted" as="span">{missing.label} is required.</Text>{/if}
		</div>
		{#if test.error && !formError}<ErrorMessage error={test.error} />{/if}
	{/if}
{/if}
{#if problem}<Text tone="danger" size="sm">{problem}</Text>{/if}

<style>
	.row {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-3);
	}
</style>
