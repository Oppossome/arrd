<script lang="ts">
	import { createQuery, keepPreviousData } from '@tanstack/svelte-query';
	import { Button, Heading, IconButton, Select, Spinner, Text, ToggleChip } from '#lib/components/atoms/index.js';
	import { EmptyState, ErrorMessage, ReleaseRow, SearchBox } from '#lib/components/molecules/index.js';
	import { queries, useMutation } from '#lib/api/query.js';
	import {
		emptyReleaseFilter,
		filterReleases,
		isFilterEmpty,
		releaseFacets,
		sortReleases,
		toggleValue,
		type ReleaseFilter,
		type ReleaseSortKey,
		type ReleaseSource
	} from '#lib/domain/releases.js';
	import { toasts } from '#lib/stores/toast.svelte.js';
	import type { ReleaseResult } from '#lib/api/types.js';

	interface Props {
		source: ReleaseSource;
		/** Search as soon as the table is shown, instead of waiting for the button. */
		auto?: boolean;
		/**
		 * A title for the results. With one, the table lays itself out as a
		 * dialog's content: the title, search and filter controls form a
		 * header that stays in view while the releases scroll.
		 */
		heading?: string;
	}
	let { source, auto = false, heading }: Props = $props();

	// The search runs once started (at once with auto). activeText is the
	// user's own query; '' means arrd's.
	let started = $state(false);
	const enabled = $derived(auto || started);
	let activeText = $state('');
	const releases = createQuery(() => ({
		...queries.releases(source, activeText),
		enabled,
		placeholderData: keepPreviousData
	}));
	const grabMovie = useMutation('grabMovie');
	const grabSeries = useMutation('grabSeries');
	const grabDiscoverMovie = useMutation('grabDiscoverMovie');
	const grabDiscoverSeries = useMutation('grabDiscoverSeries');

	// The search box starts with arrd's own query once it is known; after
	// that, the text is the user's.
	let text = $state('');
	let ownQuery = $state<string>();
	$effect(() => {
		const data = releases.data;
		if (!data || activeText !== '' || releases.isPlaceholderData || ownQuery !== undefined) return;
		ownQuery = data.query;
		text = data.query;
	});

	let sort = $state<ReleaseSortKey>('rank');
	let filter = $state<ReleaseFilter>(emptyReleaseFilter());
	let grabbing = $state<string | null>(null);

	const all = $derived(releases.data?.releases ?? []);
	const facets = $derived(releaseFacets(all));
	const acceptedCount = $derived(all.filter((r) => r.accepted).length);
	const rows = $derived(sortReleases(filterReleases(all, filter), sort));
	const filtered = $derived(!isFilterEmpty(filter));
	const keyOf = (r: ReleaseResult) => `${r.indexer_id}|${r.info_hash || r.title}`;

	function search() {
		const t = text.trim();
		const next = t === '' || t === ownQuery ? '' : t;
		if (!enabled || next !== activeText) {
			started = true;
			activeText = next;
		} else {
			void releases.refetch();
		}
	}

	function resetSearch() {
		activeText = '';
		text = ownQuery ?? '';
	}

	function searchAgain() {
		if (enabled) void releases.refetch();
		else started = true;
	}

	function grab(r: ReleaseResult) {
		grabbing = keyOf(r);
		const opts = {
			onSuccess: () => toasts.push('success', `Grabbed ${r.title}`),
			onSettled: () => (grabbing = null)
		};
		switch (source.kind) {
			case 'movie':
				return grabMovie.mutate({ id: source.movieId, release: r }, opts);
			case 'series':
			case 'season':
			case 'episode':
				return grabSeries.mutate({ id: source.seriesId, release: r }, opts);
			case 'discoverMovie':
				return grabDiscoverMovie.mutate({ tmdbId: source.tmdbId, release: r }, opts);
			case 'discoverSeries':
			case 'discoverSeason':
			case 'discoverEpisode':
				return grabDiscoverSeries.mutate({ tmdbId: source.tmdbId, release: r }, opts);
		}
	}
</script>

<section class="releases" class:framed={heading !== undefined} aria-label={heading ?? 'Releases'}>
	<header class="head">
		{#if heading !== undefined}<Heading level={3}>{heading}</Heading>{/if}

		<div class="toolbar">
			<form
				class="query"
				role="search"
				onsubmit={(e) => {
					e.preventDefault();
					search();
				}}
			>
				<div class="box">
					<SearchBox bind:value={text} label="Search indexers for" placeholder="Search indexers for…" />
				</div>
				<Button type="submit" size="sm">Search</Button>
				{#if activeText !== ''}
					<Button size="sm" variant="ghost" onclick={resetSearch}>Reset</Button>
				{/if}
			</form>
			<div class="controls">
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
				<IconButton
					icon="refresh"
					label={releases.data ? 'Search again' : 'Search indexers'}
					spinning={releases.isFetching}
					onclick={searchAgain}
				/>
			</div>
		</div>

		{#if all.length > 0}
			<div class="filters" role="group" aria-label="Filters">
				<ToggleChip
					pressed={filter.acceptedOnly}
					count={acceptedCount}
					onclick={() => (filter.acceptedOnly = !filter.acceptedOnly)}
				>
					Accepted only
				</ToggleChip>
				{#if facets.qualities.length > 1 || filter.qualities.length > 0}
					<span class="sep" aria-hidden="true"></span>
					{#each facets.qualities as f (f.value)}
						<ToggleChip
							pressed={filter.qualities.includes(f.value)}
							count={f.count}
							onclick={() => (filter.qualities = toggleValue(filter.qualities, f.value))}
						>
							{f.value}
						</ToggleChip>
					{/each}
				{/if}
				{#if facets.indexers.length > 1 || filter.indexers.length > 0}
					<span class="sep" aria-hidden="true"></span>
					{#each facets.indexers as f (f.value)}
						<ToggleChip
							pressed={filter.indexers.includes(f.value)}
							count={f.count}
							onclick={() => (filter.indexers = toggleValue(filter.indexers, f.value))}
						>
							{f.value}
						</ToggleChip>
					{/each}
				{/if}
				<span class="summary">
					<Text as="span" size="xs" tone="muted">
						{filtered ? `Showing ${rows.length} of ${all.length}` : `${all.length} results · ${acceptedCount} accepted`}
					</Text>
				</span>
			</div>
		{/if}
	</header>

	<div class="results" class:stale={releases.isPlaceholderData}>
		{#if (releases.data?.indexer_failures ?? []).length > 0}
			<div class="failures">
				{#each releases.data?.indexer_failures ?? [] as f (f.indexer_id)}
					<Text tone="warning" size="xs">{f.indexer}: {f.error}</Text>
				{/each}
			</div>
		{/if}

		{#if releases.isError}
			<ErrorMessage error={releases.error} />
		{:else if releases.isFetching && !releases.data}
			<Spinner label="Searching indexers" />
		{:else if releases.data && all.length === 0}
			<EmptyState message="No releases found. Try different search text." />
		{:else if all.length > 0 && rows.length === 0}
			<EmptyState message="No releases match these filters.">
				{#snippet action()}
					<Button size="sm" onclick={() => (filter = emptyReleaseFilter())}>Clear filters</Button>
				{/snippet}
			</EmptyState>
		{:else if rows.length > 0}
			<ul class="list" aria-label="Releases">
				{#each rows as r (keyOf(r))}
					<ReleaseRow release={r} loading={grabbing === keyOf(r)} ongrab={() => grab(r)} />
				{/each}
			</ul>
		{/if}
	</div>
</section>

<style>
	.releases,
	.results,
	.head {
		display: grid;
		gap: var(--space-3);
	}
	.toolbar {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-3);
	}
	.query {
		display: flex;
		flex: 1 1 20rem;
		align-items: center;
		gap: var(--space-2);
		min-width: 0;
	}
	.box {
		flex: 1;
		min-width: 0;
	}
	.controls {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		margin-left: auto;
	}
	.sort {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}
	.filters {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-2);
	}
	.sep {
		width: 1px;
		height: 1.25rem;
		background: var(--color-border);
	}
	.summary {
		margin-left: auto;
	}
	.failures {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-1) var(--space-4);
	}
	.list {
		margin: 0;
		padding: 0;
		list-style: none;
	}
	.stale {
		opacity: 0.5;
		transition: opacity var(--transition-fast);
	}
	/* Dialog content: a sticky header (its first row leaves room for the
	   dialog's close button), then padded results. */
	.framed {
		gap: 0;
	}
	.framed .head {
		position: sticky;
		top: 0;
		z-index: 1;
		padding: var(--space-4) var(--space-5);
		border-bottom: 1px solid var(--color-border);
		background: var(--color-surface);
	}
	.framed .head > :global(:first-child) {
		padding-right: 2.5rem;
	}
	.framed .results {
		padding: var(--space-2) var(--space-5) var(--space-5);
	}
</style>
