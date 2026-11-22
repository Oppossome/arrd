<script lang="ts">
	// Binds DiscoverSearch's query the way the discover pages do, and shows it.
	import DiscoverSearch from '#lib/components/organisms/DiscoverSearch.svelte';

	let query = $state('');
</script>

<DiscoverSearch kind="movies" bind:query />
<output data-testid="query">{query || 'none'}</output>
