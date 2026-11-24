<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { Badge, Button, Heading, Select, Spinner, Text } from '#lib/components/atoms/index.js';
	import { ConfirmDialog, ErrorMessage, FormField, SettingsForm } from '#lib/components/molecules/index.js';
	import { ApiError } from '#lib/api/client.js';
	import { timeAgo } from '#lib/utils/format.js';
	import { queries, useMutation, useVPNOptions } from '#lib/api/query.js';
	import { toasts } from '#lib/stores/toast.svelte.js';
	import type { VPNSettings, SettingValues } from '#lib/api/types.js';
	import { initialValues, missingField, toVPNInput } from '#lib/domain/settingsForm.js';

	const providers = createQuery(() => queries.vpnProviders());
	const vpn = createQuery(() => queries.vpnSettings(true));
	const setVPN = useMutation('setVPN');
	const removeVPN = useMutation('removeVPN');
	const test = useMutation('testVPN');
	const check = useMutation('testVPN');

	let editing = $state(false);
	let providerId = $state('');
	let values = $state<SettingValues>({});
	let confirmRemove = $state(false);

	const provider = $derived(providers.data?.find((p) => p.id === providerId));
	const options = useVPNOptions(() => (editing ? provider : undefined));
	// Saved secrets only carry over while the saved provider is selected.
	const secretsSet = $derived(vpn.data?.provider === providerId ? (vpn.data.secrets_set ?? []) : []);
	const missing = $derived(provider ? missingField(provider, values, secretsSet) : null);
	const formError = $derived.by(() => {
		const err = setVPN.error ?? test.error;
		return err instanceof ApiError && err.field ? { field: err.field, message: err.message } : undefined;
	});
	const otherError = $derived.by(() => {
		const err = setVPN.error ?? test.error;
		return err && !(err instanceof ApiError && err.field) ? err : null;
	});

	function choose(id: string, saved?: VPNSettings) {
		providerId = id;
		const p = providers.data?.find((d) => d.id === id);
		values = p ? initialValues(p, saved?.provider === id ? saved.values : undefined) : {};
		setVPN.reset();
		test.reset();
	}

	function startEditing(saved: VPNSettings) {
		editing = true;
		choose(saved.provider ?? providers.data?.[0]?.id ?? '', saved);
	}

	function save() {
		setVPN.mutate(toVPNInput(providerId, values), {
			onSuccess: (v) => {
				editing = false;
				toasts.push(v.tunnel_up ? 'success' : 'error', v.tunnel_up ? 'VPN connected' : `Saved, but the tunnel is down: ${v.tunnel_error ?? 'unknown'}`);
			}
		});
	}
</script>

{#if vpn.isPending || providers.isPending}
	<Spinner />
{:else if vpn.isError}
	<ErrorMessage error={vpn.error} />
{:else if providers.isError}
	<ErrorMessage error={providers.error} />
{:else}
	{@const v = vpn.data}
	{@const current = providers.data.find((p) => p.id === v.provider)}
	<section class="block">
		<Heading level={2}>Status</Heading>
		{#if !v.configured}
			<Text tone="muted">No VPN is configured. Indexer searches and downloads use your normal connection.</Text>
		{:else}
			<div class="status">
				<Badge tone={v.tunnel_up ? 'success' : 'danger'}>{v.tunnel_up ? 'Connected' : 'Down'}</Badge>
				<Text size="sm" as="span">{current?.name ?? v.provider}</Text>
				{#if v.last_handshake}<Text size="sm" tone="muted" as="span">Last handshake {timeAgo(v.last_handshake)}</Text>{/if}
			</div>
			{#if v.tunnel_error}<Text size="sm" tone="danger">{v.tunnel_error}</Text>{/if}
			{#if current?.renews}
				<Text size="sm" tone="muted">arrd renews this connection on its own and reconnects if the provider drops it.</Text>
			{/if}
			<dl class="details">
				{#if v.server}<dt>Server</dt><dd>{v.server}</dd>{/if}
				{#if v.forwarded_port}<dt>Forwarded port</dt><dd>{v.forwarded_port}</dd>{/if}
				{#if v.addresses?.length}<dt>Address</dt><dd>{v.addresses.join(', ')}</dd>{/if}
				{#if v.endpoints?.length}<dt>Endpoint</dt><dd>{v.endpoints.join(', ')}</dd>{/if}
				{#if v.tunnel_up}
					<dt>DNS</dt><dd>{v.dns?.length ? v.dns.join(', ') : 'none (hostnames cannot be resolved through the tunnel)'}</dd>
				{/if}
				{#if v.ignored_keys?.length}<dt>Ignored</dt><dd>{v.ignored_keys.join(', ')}</dd>{/if}
			</dl>
			<div class="actions">
				<Button
					loading={check.isPending}
					onclick={() => check.mutate(undefined, { onSuccess: (r) => toasts.push('success', `Traffic leaves from ${r.public_ip}`) })}
				>
					Check public IP
				</Button>
				<Button onclick={() => (editing ? (editing = false) : startEditing(v))}>{editing ? 'Cancel' : 'Change VPN'}</Button>
				<Button variant="danger" onclick={() => (confirmRemove = true)}>Remove VPN</Button>
			</div>
			{#if check.error}<ErrorMessage error={check.error} />{/if}
		{/if}
	</section>

	{#if !v.configured && !editing}
		<div class="actions">
			<Button variant="primary" onclick={() => startEditing(v)}>Set up a VPN</Button>
		</div>
	{/if}

	{#if editing}
		<section class="block">
			<Heading level={2}>{v.configured ? 'Change VPN' : 'Set up a VPN'}</Heading>
			<FormField id="vpn-provider" label="Provider">
				{#snippet children(a)}
					<Select
						id={a.id}
						value={providerId}
						options={providers.data.map((p) => ({ value: p.id, label: p.name }))}
						onchange={(e) => choose(e.currentTarget.value, v)}
					/>
				{/snippet}
			</FormField>
			{#if provider}
				{#if provider.description}<Text size="sm" tone="muted">{provider.description}</Text>{/if}
				<SettingsForm {provider} bind:values options={options.options} loading={options.loading} {secretsSet} error={formError} />
				{#if options.error}<ErrorMessage error={options.error} />{/if}
				{#if otherError}<ErrorMessage error={otherError} />{/if}
				{#if test.data}<Text size="sm" tone="success">Test OK: traffic leaves from {test.data.public_ip}</Text>{/if}
				<div class="actions">
					<Button disabled={!!missing} loading={test.isPending} onclick={() => test.mutate(toVPNInput(providerId, values))}>Test</Button>
					<Button variant="primary" disabled={!!missing} loading={setVPN.isPending} onclick={save}>Save and connect</Button>
					{#if missing}<Text size="sm" tone="muted" as="span">{missing.label} is required.</Text>{/if}
				</div>
			{/if}
		</section>
	{/if}
{/if}

<ConfirmDialog
	open={confirmRemove}
	title="Remove the VPN?"
	confirmLabel="Remove anyway"
	danger
	busy={removeVPN.isPending}
	onconfirm={() => removeVPN.mutate(undefined, { onSuccess: () => (confirmRemove = false) })}
	oncancel={() => (confirmRemove = false)}
>
	<Text size="sm" tone="warning">
		Indexer searches and downloads will use your normal connection, so trackers and peers will see your real IP address.
	</Text>
	{#if removeVPN.error}<ErrorMessage error={removeVPN.error} />{/if}
</ConfirmDialog>

<style>
	.block {
		display: grid;
		gap: var(--space-3);
	}
	.status,
	.actions {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-2);
	}
	.details {
		display: grid;
		grid-template-columns: auto 1fr;
		gap: var(--space-1) var(--space-4);
		margin: 0;
		font-size: var(--text-sm);
	}
	dt {
		color: var(--color-text-muted);
	}
	dd {
		margin: 0;
		font-family: var(--font-mono);
		word-break: break-all;
	}
</style>
