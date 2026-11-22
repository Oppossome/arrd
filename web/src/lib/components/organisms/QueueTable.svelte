<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { Checkbox, IconButton, Link, Spinner, Text } from '#lib/components/atoms/index.js';
	import { ConfirmDialog, DownloadProgress, EmptyState, ErrorMessage, StatusBadge } from '#lib/components/molecules/index.js';
	import { formatBytes, timeAgo } from '#lib/format.js';
	import { libraryHref } from '#lib/links.js';
	import { queries, useMutation, type QueueFilter } from '#lib/query.js';
	import PeerPopover from './PeerPopover.svelte';
	import type { Download } from '#lib/types.js';

	interface Props {
		filter?: QueueFilter;
		/** Hide the "what is this for" column (e.g. on a movie's own page). */
		compact?: boolean;
	}
	let { filter = {}, compact = false }: Props = $props();

	const queue = createQuery(() => queries.queue(filter));
	const remove = useMutation('removeDownload');

	let peersOf = $state<Download | null>(null);
	let removing = $state<Download | null>(null);
	let deleteData = $state(false);

	function confirmRemove() {
		if (!removing) return;
		remove.mutate({ id: removing.id, deleteData }, { onSuccess: () => (removing = null) });
	}
	const target = (d: Download) =>
		d.movie_id ? libraryHref('movies', d.movie_id) : d.series_id ? libraryHref('series', d.series_id) : undefined;
</script>

{#if queue.isPending}
	<Spinner label="Loading downloads" />
{:else if queue.isError}
	<ErrorMessage error={queue.error} />
{:else if queue.data.length === 0}
	<EmptyState message="No downloads yet." />
{:else}
	<div class="table-wrap">
		<table>
			<thead>
				<tr>
					{#if !compact}<th>For</th>{/if}
					<th>Release</th>
					<th>Status</th>
					<th>Progress</th>
					<th>Size</th>
					<th>Added</th>
					<th><span class="visually-hidden">Actions</span></th>
				</tr>
			</thead>
			<tbody>
				{#each queue.data as d (d.id)}
					<tr>
						{#if !compact}
							<td>
								{#if target(d)}
									<Link href={target(d)!}>{d.title || 'Unknown'}{d.year ? ` (${d.year})` : ''}</Link>
								{:else}
									<Text size="sm" tone="muted" as="span">Removed from library</Text>
								{/if}
								{#if d.episodes?.length}
									<Text size="xs" tone="muted">{d.episodes.length > 3 ? `${d.episodes[0]} … ${d.episodes[d.episodes.length - 1]}` : d.episodes.join(', ')}</Text>
								{/if}
							</td>
						{/if}
						<td><div class="release">
							<Text size="sm" as="span">{d.release_title}</Text>
							<Text size="xs" tone="muted" as="span">{d.quality}</Text>
							{#if d.error}<Text size="xs" tone="danger" as="span">{d.error}</Text>{/if}
						</div></td>
						<td><StatusBadge status={d.status} /></td>
						<td><DownloadProgress progress={d.progress} /></td>
						<td><Text size="sm" as="span">{formatBytes(d.size)}</Text></td>
						<td><Text size="sm" tone="muted" as="span">{timeAgo(d.added_at)}</Text></td>
						<td class="actions">
							{#if d.progress}
								<IconButton icon="users" label="Peers of {d.release_title}" onclick={() => (peersOf = d)} />
							{/if}
							<IconButton
								icon="trash"
								tone="danger"
								label="Remove {d.release_title}"
								onclick={() => {
									removing = d;
									deleteData = false;
								}}
							/>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}

<PeerPopover downloadId={peersOf?.id ?? null} title={peersOf?.release_title ?? ''} onclose={() => (peersOf = null)} />

<ConfirmDialog
	open={removing !== null}
	title="Remove download"
	confirmLabel="Remove"
	danger
	busy={remove.isPending}
	onconfirm={confirmRemove}
	oncancel={() => (removing = null)}
>
	<Text size="sm">Stop and forget <strong>{removing?.release_title}</strong>? Files already in your library are kept.</Text>
	<Checkbox bind:checked={deleteData} label="Also delete the downloaded data" />
</ConfirmDialog>

<style>
	.table-wrap {
		overflow-x: auto;
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
		vertical-align: top;
	}
	th {
		color: var(--color-text-muted);
		font-size: var(--text-xs);
		font-weight: var(--weight-medium);
	}
	.actions {
		white-space: nowrap;
	}
	.release {
		display: grid;
		gap: var(--space-1);
		min-width: 16rem;
	}
</style>
