<script lang="ts">
	import { createQuery, keepPreviousData } from '@tanstack/svelte-query';
	import { Button, Heading, Spinner, Text, ToggleChip } from '#lib/components/atoms/index.js';
	import { ChoiceMenu, EmptyState, ErrorMessage, FacetMenu, ReleaseRow, SearchBox } from '#lib/components/molecules/index.js';
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
		type ReleaseTarget
	} from '#lib/domain/releases.js';
	import { toasts } from '#lib/stores/toast.svelte.js';
	import type { ReleaseResult } from '#lib/api/types.js';

	interface Props {
		target: ReleaseTarget;
		/** Search as soon as the table is shown, instead of waiting for the button. */
		auto?: boolean;
		/**
		 * A title for the results. With one, the table lays itself out as a
		 * dialog's content: the title, search and filter controls form a
		 * header that stays in view while the releases scroll.
		 */
		heading?: string;
	}
	let { target, auto = false, heading }: Props = $props();

	// The search runs once started (at once with auto). activeText is the
	// user's own query; '' means arrd's.
	let started = $state(false);
	const enabled = $derived(auto || started);
	let activeText = $state('');
	const releases = createQuery(() => ({
		...queries.releases(target, activeText),
		enabled,
		placeholderData: keepPreviousData
	}));
	const grabRelease = useMutation('grab');

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

	function grab(r: ReleaseResult) {
		grabbing = keyOf(r);
		const opts = {
			onSuccess: () => toasts.push('success', `Grabbed ${r.title}`),
			onSettled: () => (grabbing = null)
		};
		grabRelease.mutate({ target, release: r }, opts);
	}
</script>

<section class="releases" class:framed={heading !== undefined} aria-label={heading ?? 'Releases'}>
	<header class="head">
		{#if heading !== undefined}<Heading level={3}>{heading}</Heading>{/if}

		<!-- Search runs the indexer search; pressed again with the same text,
		     it searches again. -->
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
			<Button type="submit" size="sm" loading={releases.isFetching}>Search</Button>
			{#if activeText !== ''}
				<Button size="sm" variant="ghost" onclick={resetSearch}>Reset</Button>
			{/if}
		</form>

		{#if all.length > 0}
			<div class="filters" role="group" aria-label="Filters">
				<ToggleChip
					pressed={filter.acceptedOnly}
					count={acceptedCount}
					onclick={() => (filter.acceptedOnly = !filter.acceptedOnly)}
				>
					Accepted
				</ToggleChip>
				{#if facets.qualities.length > 1 || filter.qualities.length > 0}
					<FacetMenu
						label="Quality"
						facets={facets.qualities}
						selected={filter.qualities}
						ontoggle={(v) => (filter.qualities = toggleValue(filter.qualities, v))}
						onclear={() => (filter.qualities = [])}
					/>
				{/if}
				{#if facets.indexers.length > 1 || filter.indexers.length > 0}
					<FacetMenu
						label="Indexer"
						facets={facets.indexers}
						selected={filter.indexers}
						ontoggle={(v) => (filter.indexers = toggleValue(filter.indexers, v))}
						onclear={() => (filter.indexers = [])}
					/>
				{/if}
				<span class="sort">
					<ChoiceMenu
						label="Sort"
						bind:value={sort}
						options={[
							{ value: 'rank', label: 'Best match' },
							{ value: 'quality', label: 'Best quality' },
							{ value: 'seeders', label: 'Most seeders' },
							{ value: 'size', label: 'Largest' },
							{ value: 'age', label: 'Newest' }
						]}
					/>
				</span>
			</div>
		{/if}
	</header>

	<div class="results" class:stale={releases.isPlaceholderData}>
		{#if all.length > 0}
			<Text size="xs" tone="muted">
				{filtered ? `Showing ${rows.length} of ${all.length} results` : `${all.length} results`}
			</Text>
		{/if}
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
	.query {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		min-width: 0;
	}
	.box {
		flex: 1;
		min-width: 0;
	}
	/* One line that scrolls sideways when narrow. The menus' panels open in
	   the top layer, so the scroll box does not clip them. The vertical
	   padding keeps focus rings inside the box. */
	.filters {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		margin: calc(-1 * var(--space-1));
		padding: var(--space-1);
		overflow-x: auto;
		scrollbar-width: none;
	}
	.filters > :global(*) {
		flex-shrink: 0;
	}
	/* Sort sits at the row's end. */
	.sort {
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
