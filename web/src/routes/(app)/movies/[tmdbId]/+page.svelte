<script lang="ts">
	import { page } from '$app/state';
	import { DiscoverHeader, MoreLikeThis, ReleaseTable } from '#lib/components/organisms/index.js';
	import { DetailLayout, Section } from '#lib/components/templates/index.js';

	const tmdbId = $derived(Number(page.params.tmdbId));
</script>

{#key tmdbId}
	<DetailLayout>
		{#snippet header()}<DiscoverHeader kind="movies" {tmdbId} />{/snippet}
		<MoreLikeThis kind="movies" {tmdbId} />
		<Section title="Sources" description="Choose one to download. The movie is added to your library when you do.">
			<ReleaseTable source={{ kind: 'discoverMovie', tmdbId }} auto />
		</Section>
	</DetailLayout>
{/key}
