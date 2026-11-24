<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { Heading, Link, Spinner, Text, Toggle } from '#lib/components/atoms/index.js';
	import { ErrorMessage } from '#lib/components/molecules/index.js';
	import { queries, useMutation } from '#lib/api/query.js';
	import { toasts } from '#lib/stores/toast.svelte.js';

	const network = createQuery(() => queries.networkSettings());
	const updateNetwork = useMutation('updateNetwork');

	function setExposeLAN(on: boolean) {
		updateNetwork.mutate(on, {
			onSuccess: () => toasts.push('success', on ? 'Available on your local network' : 'Local network access turned off')
		});
	}
</script>

{#if network.isPending}
	<Spinner label="Loading network settings" />
{:else if network.isError}
	<ErrorMessage error={network.error} />
{:else}
	<section class="section" aria-label="Local network">
		<div class="head">
			<Heading level={2}>Local network</Heading>
			<Text size="sm" tone="muted">
				Serve arrd to other devices on your network and announce it as {network.data.hostname}. Signing in is still required.
			</Text>
		</div>
		<div class="row">
			<Text as="span">Allow access from other devices on your network</Text>
			<Toggle
				checked={network.data.expose_lan}
				label="Allow access from other devices on your network"
				disabled={updateNetwork.isPending}
				onchange={setExposeLAN}
			/>
		</div>
		{#if network.data.lan_url}
			<Text size="sm">Open <Link href={network.data.lan_url}>{network.data.lan_url}</Link> from another device.</Text>
		{/if}
		{#if updateNetwork.error}<ErrorMessage error={updateNetwork.error} />{/if}
	</section>
{/if}

<style>
	.section {
		display: grid;
		gap: var(--space-4);
	}
	.head {
		display: grid;
		gap: var(--space-1);
	}
	.row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-3);
	}
</style>
