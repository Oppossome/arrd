<script lang="ts">
	import { page } from '$app/state';
	import { DiscoverHeader, MoreLikeThis, ReleaseTable } from '#lib/components/organisms/index.js';
	import { DetailLayout, Section } from '#lib/components/templates/index.js';

	const tmdbId = $derived(Number(page.params.tmdbId));
	let season = $state<number | undefined>();
</script>

{#key tmdbId}
	<DetailLayout>
		{#snippet header()}<DiscoverHeader kind="series" {tmdbId} bind:season />{/snippet}
		<MoreLikeThis kind="series" {tmdbId} />
		{#if season !== undefined}
			<Section
				title={season === 0 ? 'Specials sources' : `Season ${season} sources`}
				description="Season packs and single episodes. The show is added to your library when you download one."
			>
				{#key season}<ReleaseTable source={{ kind: 'discoverSeason', tmdbId, season }} auto />{/key}
			</Section>
		{/if}
	</DetailLayout>
{/key}
