<script lang="ts">
	import { createInfiniteQuery } from '@tanstack/svelte-query';
	import { Button, Label, Select, Skeleton, Spinner } from '#lib/components/atoms/index.js';
	import { EmptyState, ErrorMessage, PosterCard, PosterWall } from '#lib/components/molecules/index.js';
	import { posterCardProps, sortOptions } from '#lib/discover.js';
	import type { MediaKind } from '#lib/links.js';
	import { queries } from '#lib/query.js';
	import type { DiscoverSort, LookupMovie, LookupSeries, TitleFilter } from '#lib/types.js';

	interface Props {
		kind: MediaKind;
		/** The genres and language to browse. */
		filter: TitleFilter;
		sort: DiscoverSort;
		/** The user picked another order. */
		onsortchange: (sort: DiscoverSort) => void;
	}
	let { kind, filter, sort, onsortchange }: Props = $props();

	const pages = createInfiniteQuery(() => queries.browse(kind, filter, sort));
	// Popularity shifts between page loads, so a title can reappear on a later page.
	const items = $derived(
		(pages.data?.pages ?? [])
			.flatMap((p): (LookupMovie | LookupSeries)[] => p.items)
			.filter((it, i, all) => all.findIndex((o) => o.tmdb_id === it.tmdb_id) === i)
	);

	// Infinite scroll: fetch the next page as the end of the grid nears the
	// viewport. Observing again after each page makes the observer report the
	// sentinel's current state, so a page too short to fill the screen still
	// leads to the next one. After a failed page it waits for a retry.
	let sentinel = $state<HTMLDivElement>();
	$effect(() => {
		void pages.data?.pages.length;
		if (!sentinel) return;
		const observer = new IntersectionObserver(
			(entries) => {
				if (!entries.some((e) => e.isIntersecting)) return;
				if (pages.hasNextPage && !pages.isFetching && !pages.isFetchNextPageError) void pages.fetchNextPage();
			},
			{ rootMargin: '0px 0px 600px 0px' }
		);
		observer.observe(sentinel);
		return () => observer.disconnect();
	});
</script>

<div class="browse">
	<div class="controls">
		<div class="control">
			<Label for="browse-sort">Order</Label>
			<Select
				id="browse-sort"
				value={sort}
				options={sortOptions}
				onchange={(e) => onsortchange(e.currentTarget.value as DiscoverSort)}
			/>
		</div>
	</div>

	{#if pages.isPending}
		<PosterWall>{#each Array(20) as _, i (i)}<Skeleton height="16rem" />{/each}</PosterWall>
	{:else if pages.isError && items.length === 0}
		<ErrorMessage error={pages.error} />
	{:else if items.length === 0}
		<EmptyState message="TMDB has nothing here." />
	{:else}
		<PosterWall>
			{#each items as item (item.tmdb_id)}<PosterCard {...posterCardProps(kind, item)} />{/each}
		</PosterWall>
		{#if pages.isFetchNextPageError}
			<div class="more">
				<ErrorMessage error={pages.error} />
				<Button onclick={() => pages.fetchNextPage()}>Retry</Button>
			</div>
		{:else if pages.hasNextPage}
			<div class="more" bind:this={sentinel}>
				{#if pages.isFetchingNextPage}<Spinner label="Loading more" />{/if}
			</div>
		{/if}
	{/if}
</div>

<style>
	.browse {
		display: grid;
		gap: var(--space-5);
	}
	.controls {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-4);
	}
	.control {
		display: grid;
		gap: var(--space-1);
		min-width: 12rem;
	}
	.more {
		display: flex;
		flex-wrap: wrap;
		justify-content: center;
		align-items: center;
		gap: var(--space-3);
		min-height: 2.5rem;
	}
</style>
