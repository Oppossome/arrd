<script lang="ts">
	import { page } from '$app/state';
	import { EpisodeTable, QueueTable, ReleaseTable, SeasonList, SeriesHeader } from '#lib/components/organisms/index.js';
	import { DetailLayout, Section } from '#lib/components/templates/index.js';
	import { episodeCode } from '#lib/format.js';
	import type { Episode } from '#lib/types.js';

	const id = $derived(Number(page.params.id));
	let episode = $state<Episode | null>(null);
</script>

{#key id}
	<DetailLayout>
		{#snippet header()}<SeriesHeader {id} />{/snippet}
		<Section title="Seasons">
			<SeasonList seriesId={id}>
				{#snippet season(s)}
					<EpisodeTable seriesId={id} season={s.season} onreleases={(e) => (episode = e)} />
					<Section title="Season releases" description="Packs and episodes for this season.">
						<ReleaseTable source={{ kind: 'season', seriesId: id, season: s.season }} />
					</Section>
				{/snippet}
			</SeasonList>
		</Section>
		{#if episode}
			{#key episode.id}
				<Section title="Releases for {episodeCode(episode.season, episode.episode)} {episode.title}">
					<ReleaseTable source={{ kind: 'episode', seriesId: id, episodeId: episode.id }} />
				</Section>
			{/key}
		{/if}
		<Section title="Downloads">
			<QueueTable filter={{ seriesId: id }} compact />
		</Section>
	</DetailLayout>
{/key}
