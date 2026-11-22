<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { Badge, Button, Select, Spinner, Text } from '#lib/components/atoms/index.js';
	import { EmptyState, ErrorMessage } from '#lib/components/molecules/index.js';
	import { formatBytes, timeAgo } from '#lib/format.js';
	import { queries, useMutation, type ReleaseSource } from '#lib/query.js';
	import { releaseCoverage, sortReleases, type ReleaseSortKey } from '#lib/releases.js';
	import { toasts } from '#lib/toast.svelte.js';
	import type { ReleaseResult } from '#lib/types.js';

	interface Props {
		source: ReleaseSource;
		/** Search as soon as the table is shown, instead of waiting for the button. */
		auto?: boolean;
	}
	let { source, auto = false }: Props = $props();

	const releases = createQuery(() => ({ ...queries.releases(source), enabled: auto }));
	const grabMovie = useMutation('grabMovie');
	const grabSeries = useMutation('grabSeries');
	const grabDiscoverMovie = useMutation('grabDiscoverMovie');
	const grabDiscoverSeries = useMutation('grabDiscoverSeries');

	let sort = $state<ReleaseSortKey>('rank');
	let grabbing = $state<string | null>(null);
	const rows = $derived(sortReleases(releases.data?.releases ?? [], sort));
	const keyOf = (r: ReleaseResult) => `${r.indexer_id}|${r.info_hash || r.title}`;

	function grab(r: ReleaseResult) {
		grabbing = keyOf(r);
		const opts = {
			onSuccess: () => toasts.push('success', `Grabbed ${r.title}`),
			onSettled: () => (grabbing = null)
		};
		switch (source.kind) {
			case 'movie':
				return grabMovie.mutate({ id: source.movieId, release: r }, opts);
			case 'season':
			case 'episode':
				return grabSeries.mutate({ id: source.seriesId, release: r }, opts);
			case 'discoverMovie':
				return grabDiscoverMovie.mutate({ tmdbId: source.tmdbId, release: r }, opts);
			case 'discoverSeason':
				return grabDiscoverSeries.mutate({ tmdbId: source.tmdbId, release: r }, opts);
		}
	}
</script>

<section class="releases" aria-label="Releases">
	<div class="toolbar">
		<Button variant="primary" loading={releases.isFetching} onclick={() => releases.refetch()}>
			{releases.data ? 'Search again' : 'Search indexers'}
		</Button>
		{#if releases.data}
			<label class="sort">
				<Text as="span" size="sm" tone="muted">Sort</Text>
				<Select
					bind:value={sort}
					options={[
						{ value: 'rank', label: 'Best match' },
						{ value: 'quality', label: 'Quality' },
						{ value: 'seeders', label: 'Seeders' },
						{ value: 'size', label: 'Size' },
						{ value: 'age', label: 'Newest' }
					]}
				/>
			</label>
		{/if}
	</div>

	{#each releases.data?.indexer_failures ?? [] as f (f.indexer_id)}
		<Text tone="warning" size="sm">{f.indexer}: {f.error}</Text>
	{/each}

	{#if releases.isError}
		<ErrorMessage error={releases.error} />
	{:else if releases.isFetching && !releases.data}
		<Spinner label="Searching indexers" />
	{:else if releases.data && rows.length === 0}
		<EmptyState message="No releases found." />
	{:else if rows.length > 0}
		<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th>Release</th>
						<th>Quality</th>
						<th>Size</th>
						<th>Seeders</th>
						<th>Indexer</th>
						<th>Age</th>
						<th><span class="visually-hidden">Actions</span></th>
					</tr>
				</thead>
				<tbody>
					{#each rows as r (keyOf(r))}
						<tr class:rejected={!r.accepted}>
							<td><div class="title">
								<Text size="sm" as="span">{r.title}</Text>
								{#if releaseCoverage(r)}<Badge tone="info">{releaseCoverage(r)}</Badge>{/if}
								{#if !r.accepted}
									<ul class="reasons">
										{#each r.rejections ?? [] as why (why)}<li><Text size="xs" tone="danger" as="span">{why}</Text></li>{/each}
									</ul>
								{/if}
							</div></td>
							<td><Badge tone={r.accepted ? 'success' : 'neutral'}>{r.quality}</Badge></td>
							<td><Text size="sm" as="span">{formatBytes(r.size)}</Text></td>
							<td><Text size="sm" as="span">{r.seeders}</Text></td>
							<td><Text size="sm" as="span" tone="muted">{r.indexer}</Text></td>
							<td><Text size="sm" as="span" tone="muted">{timeAgo(r.publish_date)}</Text></td>
							<td>
								<Button size="sm" variant={r.accepted ? 'primary' : 'secondary'} loading={grabbing === keyOf(r)} onclick={() => grab(r)}>
									{r.accepted ? 'Grab' : 'Grab anyway'}
								</Button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</section>

<style>
	.releases {
		display: grid;
		gap: var(--space-3);
	}
	.toolbar {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-3);
	}
	.sort {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}
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
	.title {
		display: grid;
		gap: var(--space-1);
		justify-items: start;
		min-width: 18rem;
	}
	.rejected {
		opacity: 0.7;
	}
	.reasons {
		margin: 0;
		padding-left: var(--space-4);
	}
</style>
