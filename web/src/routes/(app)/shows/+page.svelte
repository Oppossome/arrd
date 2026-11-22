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

<ListLayout title="Shows">
	<DiscoverSearch kind="series" bind:query />
	{#if query}
		<DiscoverResults kind="series" {query} />
	{:else}
		<GenreChips kind="series" />
		<RecommendationRows kind="series" />
		{#each discoverRows('series') as row (row.list)}
			<DiscoverListRow kind="series" list={row.list} title={row.title} />
		{/each}
	{/if}
</ListLayout>
