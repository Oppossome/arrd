<script lang="ts">
	// Binds DiscoverHeader's season the way the series page does, and shows it.
	import DiscoverHeader from '#lib/components/organisms/DiscoverHeader.svelte';

	let { tmdbId }: { tmdbId: number } = $props();
	let season = $state<number | undefined>();
</script>

<DiscoverHeader kind="series" {tmdbId} bind:season />
<output data-testid="season">{season ?? 'none'}</output>
