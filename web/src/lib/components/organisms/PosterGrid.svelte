<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { Skeleton } from '#lib/components/atoms/index.js';
	import { EmptyState, ErrorMessage, PosterCard, SearchBox, Tabs } from '#lib/components/molecules/index.js';
	import { posterURL } from '#lib/api/client.js';
	import { itemDownloads, type ItemDownload } from '#lib/domain/downloads.js';
	import { libraryHref } from '#lib/nav/links.js';
	import { queries } from '#lib/api/query.js';
	import type { MovieStatus } from '#lib/api/types.js';

	interface Props {
		kind: 'movies' | 'series';
	}
	let { kind }: Props = $props();

	type Filter = 'all' | MovieStatus | 'failed';
	let filter = $state<Filter>('all');
	let search = $state('');

	const movies = createQuery(() => ({ ...queries.movies(), enabled: kind === 'movies' }));
	const series = createQuery(() => ({ ...queries.seriesList(), enabled: kind === 'series' }));
	// The unfiltered queue, shared with the active-downloads shelf.
	const queue = createQuery(() => queries.queue());
	const downloads = $derived(itemDownloads(queue.data ?? []));

	interface Item {
		id: number;
		title: string;
		year: number;
		poster?: string;
		status?: MovieStatus;
		subtitle?: string;
		download?: ItemDownload;
	}

	const items = $derived.by((): Item[] => {
		if (kind === 'movies') {
			return (movies.data ?? []).map((m) => ({
				id: m.id,
				title: m.title,
				year: m.year,
				poster: m.poster_path,
				status: m.status,
				download: downloads.movies.get(m.id)
			}));
		}
		return (series.data ?? []).map((s) => {
			const have = s.seasons.reduce((n, x) => n + x.downloaded, 0);
			const aired = s.seasons.filter((x) => x.season > 0).reduce((n, x) => n + x.aired, 0);
			return {
				id: s.id,
				title: s.title,
				year: s.year,
				poster: s.poster_path,
				subtitle: `${have}/${aired} episodes`,
				download: downloads.series.get(s.id)
			};
		});
	});
	const matches = (i: Item): boolean => {
		switch (filter) {
			case 'all':
				return true;
			case 'downloading':
				return i.download?.kind === 'active' || i.status === 'downloading';
			case 'failed':
				return i.download?.kind === 'failed';
			default:
				return i.status === filter;
		}
	};
	const visible = $derived(items.filter((i) => matches(i) && i.title.toLowerCase().includes(search.trim().toLowerCase())));
	const q = $derived(kind === 'movies' ? movies : series);
	const filters = $derived<{ key: Filter; label: string }[]>(
		kind === 'movies'
			? [
					{ key: 'all', label: 'All' },
					{ key: 'missing', label: 'Missing' },
					{ key: 'downloading', label: 'Downloading' },
					{ key: 'downloaded', label: 'Downloaded' },
					{ key: 'failed', label: 'Failed' }
				]
			: [
					{ key: 'all', label: 'All' },
					{ key: 'downloading', label: 'Downloading' },
					{ key: 'failed', label: 'Failed' }
				]
	);
</script>

<div class="grid-wrap">
	<div class="controls">
		<Tabs tabs={filters} active={filter} label="Filter {kind === 'movies' ? 'movies' : 'shows'}" onselect={(k) => (filter = k)} />
		<SearchBox bind:value={search} label="Filter by title" placeholder="Filter…" />
	</div>

	{#if q.isPending}
		<div class="grid">
			{#each Array(12) as _, i (i)}<Skeleton height="16rem" />{/each}
		</div>
	{:else if q.isError}
		<ErrorMessage error={q.error} />
	{:else if items.length === 0}
		<EmptyState message={kind === 'movies' ? 'No movies yet. Find one under Movies.' : 'No shows yet. Find one under Shows.'} />
	{:else if visible.length === 0}
		<EmptyState message="Nothing matches this filter." />
	{:else}
		<div class="grid">
			{#each visible as item (item.id)}
				<PosterCard
					href={libraryHref(kind, item.id)}
					title={item.title}
					year={item.year}
					posterSrc={posterURL(item.poster, 'w342')}
					status={item.status}
					subtitle={item.subtitle}
					download={item.download}
				/>
			{/each}
		</div>
	{/if}
</div>
<style>
	.grid-wrap {
		display: grid;
		gap: var(--space-4);
	}
	.controls {
		display: flex;
		flex-wrap: wrap;
		justify-content: space-between;
		align-items: end;
		gap: var(--space-3);
	}
	.grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(9.5rem, 1fr));
		gap: var(--space-5) var(--space-4);
	}
</style>
