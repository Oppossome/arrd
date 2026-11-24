<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { Heading, Skeleton } from '#lib/components/atoms/index.js';
	import { EmptyState, ErrorMessage, PosterCard, PosterWall } from '#lib/components/molecules/index.js';
	import { posterCardProps } from '#lib/domain/discover.js';
	import type { MediaKind } from '#lib/nav/links.js';
	import { queries } from '#lib/api/query.js';
	import type { TitleFilter } from '#lib/api/types.js';

	interface Props {
		kind: MediaKind;
		/** What to search TMDB for. */
		query: string;
		/** Keeps only results with these genres and language; every result when unset. */
		filter?: TitleFilter;
	}
	let { kind, query, filter }: Props = $props();

	const results = createQuery(() => queries.lookup(kind, query, filter));
</script>

<section class="results">
	<Heading level={2}>Results for “{query}”</Heading>
	{#if results.isPending}
		<PosterWall>{#each Array(10) as _, i (i)}<Skeleton height="16rem" />{/each}</PosterWall>
	{:else if results.isError}
		<ErrorMessage error={results.error} />
	{:else if results.data.length === 0}
		<EmptyState message="No {kind === 'movies' ? 'movies' : 'shows'} match that search." />
	{:else}
		<PosterWall>
			{#each results.data as item (item.tmdb_id)}<PosterCard {...posterCardProps(kind, item)} />{/each}
		</PosterWall>
	{/if}
</section>

<style>
	.results {
		display: grid;
		gap: var(--space-4);
	}
</style>
