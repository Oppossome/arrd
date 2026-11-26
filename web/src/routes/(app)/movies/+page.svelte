<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import {
		BrowseGrid,
		DiscoverFilterBar,
		DiscoverListRow,
		DiscoverResults,
		RecommendationRows
	} from '#lib/components/organisms/index.js';
	import { ListLayout } from '#lib/components/templates/index.js';
	import { DEFAULT_SORT, discoverRows, parseSort } from '#lib/domain/discover.js';
	import { hasFilter, parseFacets, type Facets } from '#lib/domain/facets.js';
	import { browseHref } from '#lib/nav/links.js';
	import type { DiscoverSort } from '#lib/api/types.js';

	const facets = $derived(parseFacets(page.url.searchParams));
	const sort = $derived(parseSort(page.url.searchParams.get('sort')));

	function show(next: Facets, nextSort: DiscoverSort = sort) {
		const href = browseHref('movies', { ...next, sort: nextSort === DEFAULT_SORT ? undefined : nextSort });
		// Keep focus in the search bar (or on the order select) the user just used.
		void goto(href, { reset: false });
	}
</script>

<ListLayout title="Movies">
	<DiscoverFilterBar kind="movies" {facets} onchange={(next) => show(next)} />
	{#if facets.title !== undefined}
		<DiscoverResults kind="movies" query={facets.title} filter={facets} />
	{:else if hasFilter(facets)}
		<BrowseGrid kind="movies" filter={facets} {sort} onsortchange={(next) => show(facets, next)} />
	{:else}
		<RecommendationRows kind="movies" />
		{#each discoverRows('movies') as row (row.list)}
			<DiscoverListRow kind="movies" list={row.list} title={row.title} />
		{/each}
	{/if}
</ListLayout>
