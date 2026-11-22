<script lang="ts">
	import {
		DiscoverListRow,
		DiscoverResults,
		DiscoverSearch,
		GenreChips,
		RecommendationRows
	} from '#lib/components/organisms/index.js';
	import { ListLayout } from '#lib/components/templates/index.js';
	import { discoverRows } from '#lib/discover.js';

	let query = $state('');
</script>

<ListLayout title="Movies">
	<DiscoverSearch kind="movies" bind:query />
	{#if query}
		<DiscoverResults kind="movies" {query} />
	{:else}
		<GenreChips kind="movies" />
		<RecommendationRows kind="movies" />
		{#each discoverRows('movies') as row (row.list)}
			<DiscoverListRow kind="movies" list={row.list} title={row.title} />
		{/each}
	{/if}
</ListLayout>
