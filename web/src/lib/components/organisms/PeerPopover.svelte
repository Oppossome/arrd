<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { CountryFlag, Heading, IconButton, Spinner, Text } from '#lib/components/atoms/index.js';
	import { EmptyState, ErrorMessage } from '#lib/components/molecules/index.js';
	import { formatRate } from '#lib/utils/format.js';
	import { queries } from '#lib/api/query.js';

	interface Props {
		/** The download whose peers to show; null when closed. */
		downloadId: number | null;
		title: string;
		onclose: () => void;
	}
	let { downloadId, title, onclose }: Props = $props();

	const open = $derived(downloadId !== null);
	// Polls only while open.
	const peers = createQuery(() => ({ ...queries.downloadPeers(downloadId ?? 0), enabled: downloadId !== null }));

	const sourceLabel: Record<string, string> = {
		tracker: 'Tracker',
		dht: 'DHT',
		pex: 'PEX',
		incoming: 'Incoming',
		lpd: 'Local',
		holepunch: 'Holepunch',
		direct: 'Direct',
		'': '—'
	};
	// IPv6 addresses need brackets before a port.
	const hostPort = (ip: string, port: number) => (ip.includes(':') ? `[${ip}]:${port}` : `${ip}:${port}`);

	let dialog = $state<HTMLDialogElement>();
	$effect(() => {
		if (!dialog) return;
		if (open && !dialog.open) {
			if (typeof dialog.showModal === 'function') dialog.showModal();
			else dialog.setAttribute('open', '');
		}
		if (!open && dialog.open) {
			if (typeof dialog.close === 'function') dialog.close();
			else dialog.removeAttribute('open');
		}
	});
</script>

<dialog bind:this={dialog} aria-label="Peers of {title}" oncancel={(e) => { e.preventDefault(); onclose(); }}>
	{#if open}
		<div class="body">
			<header>
				<div>
					<Heading level={3}>Peers</Heading>
					<Text size="sm" tone="muted">{title}</Text>
				</div>
				<IconButton icon="x" label="Close" onclick={onclose} />
			</header>
			{#if peers.isPending}
				<Spinner label="Loading peers" />
			{:else if peers.isError}
				<ErrorMessage error={peers.error} />
			{:else if peers.data.length === 0}
				<EmptyState message="Not connected to any peers right now." />
			{:else}
				<div class="table-wrap">
					<table>
						<thead>
							<tr>
								<th>Country</th>
								<th>Address</th>
								<th>Client</th>
								<th>Source</th>
								<th class="num">↓</th>
								<th class="num">↑</th>
								<th class="num">Has</th>
							</tr>
						</thead>
						<tbody>
							{#each peers.data as p (hostPort(p.ip, p.port))}
								<tr>
									<td><CountryFlag code={p.country} showCode /></td>
									<td><Text size="sm" mono as="span">{hostPort(p.ip, p.port)}</Text></td>
									<td><Text size="sm" as="span">{p.client || '—'}</Text></td>
									<td><Text size="sm" tone="muted" as="span">{sourceLabel[p.source] ?? p.source}</Text></td>
									<td class="num"><Text size="sm" as="span">{formatRate(p.download_rate)}</Text></td>
									<td class="num"><Text size="sm" as="span">{formatRate(p.upload_rate)}</Text></td>
									<td class="num"><Text size="sm" as="span">{Math.floor(p.progress * 100)}%</Text></td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
			<Text size="xs" tone="muted">
				<a href="https://db-ip.com" target="_blank" rel="noopener noreferrer">IP Geolocation by DB-IP</a>
			</Text>
		</div>
	{/if}
</dialog>

<style>
	dialog {
		padding: 0;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-lg);
		background: var(--color-surface);
		color: var(--color-text);
		box-shadow: var(--shadow-raised);
		width: min(52rem, 94vw);
	}
	dialog::backdrop {
		background: var(--color-overlay);
	}
	.body {
		display: grid;
		gap: var(--space-4);
		padding: var(--space-5);
	}
	header {
		display: flex;
		justify-content: space-between;
		align-items: start;
		gap: var(--space-3);
	}
	.table-wrap {
		overflow: auto;
		max-height: 60vh;
	}
	table {
		width: 100%;
		border-collapse: collapse;
	}
	th,
	td {
		padding: var(--space-2);
		border-bottom: 1px solid var(--color-border);
		text-align: left;
		white-space: nowrap;
	}
	th {
		position: sticky;
		top: 0;
		background: var(--color-surface);
		color: var(--color-text-muted);
		font-size: var(--text-xs);
		font-weight: var(--weight-medium);
	}
	.num {
		text-align: right;
	}
	a {
		color: inherit;
	}
</style>
