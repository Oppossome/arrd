<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { Badge, Button, Heading, Icon, Spinner, Text } from '#lib/components/atoms/index.js';
	import { EmptyState, EpisodeCard, ErrorMessage, MonitorToggle, Tabs } from '#lib/components/molecules/index.js';
	import { stillURL } from '#lib/api/client.js';
	import { episodeCode } from '#lib/utils/format.js';
	import { queries, useMutation } from '#lib/api/query.js';
	import type { ReleaseSource, SourcesRequest } from '#lib/domain/releases.js';
	import type { Tone } from '#lib/domain/status.js';
	import { toasts } from '#lib/stores/toast.svelte.js';
	import type { Episode, LookupEpisode } from '#lib/api/types.js';

	interface Props {
		/** The TMDB series to show... */
		tmdbId?: number;
		/** ...or a library series. A TMDB series that is in the library is shown as one too. */
		seriesId?: number;
		/** Show the sources to download a season or an episode from. */
		ondownload: (request: SourcesRequest) => void;
	}
	let { tmdbId, seriesId, ondownload }: Props = $props();

	const libSeries = createQuery(() => ({ ...queries.series(seriesId ?? 0), enabled: seriesId !== undefined }));
	const resolvedTMDBId = $derived(tmdbId ?? libSeries.data?.tmdb_id);
	const show = createQuery(() => ({ ...queries.discoverShow(resolvedTMDBId ?? 0), enabled: !!resolvedTMDBId }));
	/** The library series, if the show is in the library. */
	const librarySeriesId = $derived(seriesId ?? show.data?.library_id);
	const series = createQuery(() => ({
		...queries.series(librarySeriesId ?? 0),
		enabled: librarySeriesId !== undefined
	}));
	const libraryEpisodes = createQuery(() => ({
		...queries.episodes(librarySeriesId ?? 0),
		enabled: librarySeriesId !== undefined
	}));
	const title = $derived(show.data?.title ?? series.data?.title ?? '');

	interface SeasonTab {
		number: number;
		episodes: number;
		name?: string;
		airDate?: string;
	}
	// Seasons in order, specials last. TMDB lists them; the library is the fallback.
	const seasons = $derived.by((): SeasonTab[] => {
		const fromTMDB = (show.data?.seasons ?? []).map((s) => ({
			number: s.season_number,
			episodes: s.episode_count,
			name: s.name,
			airDate: s.air_date
		}));
		const list = fromTMDB.length
			? fromTMDB
			: (series.data?.seasons ?? []).map((s) => ({ number: s.season, episodes: s.episodes }));
		return list
			.filter((s) => s.episodes > 0)
			.toSorted((a, b) => (a.number === 0 ? 1 : b.number === 0 ? -1 : a.number - b.number));
	});

	let picked = $state<number | undefined>();
	// The latest regular season until the user picks one.
	const season = $derived(
		picked !== undefined && seasons.some((s) => s.number === picked)
			? picked
			: (seasons.findLast((s) => s.number !== 0) ?? seasons[0])?.number
	);
	const current = $derived(seasons.find((s) => s.number === season));
	const seasonLabel = (n: number) => (n === 0 ? 'Specials' : `Season ${n}`);

	const details = createQuery(() => ({
		...queries.discoverSeason(resolvedTMDBId ?? 0, season ?? 0),
		enabled: !!resolvedTMDBId && season !== undefined
	}));
	const seasonStatus = $derived(series.data?.seasons.find((s) => s.season === season));

	interface Row {
		number: number;
		meta?: LookupEpisode;
		library?: Episode;
	}
	// TMDB's episodes (with stills and synopses), matched to library episodes by number.
	const rows = $derived.by((): Row[] => {
		const lib = new Map((libraryEpisodes.data ?? []).filter((e) => e.season === season).map((e) => [e.episode, e]));
		const meta = details.data?.episodes ?? [];
		const numbers = new Set([...meta.map((e) => e.episode), ...lib.keys()]);
		const byNumber = new Map(meta.map((e) => [e.episode, e]));
		return [...numbers].sort((a, b) => a - b).map((n) => ({ number: n, meta: byNumber.get(n), library: lib.get(n) }));
	});
	const loadingEpisodes = $derived(
		(details.isPending && details.fetchStatus !== 'idle') || (librarySeriesId !== undefined && libraryEpisodes.isPending)
	);

	const today = new Date().toISOString().slice(0, 10);
	function episodeState(e: Episode): { label: string; tone: Tone } {
		if (e.file_path) return { label: e.file_quality ?? 'Downloaded', tone: 'success' };
		if (e.downloading) return { label: 'Downloading', tone: 'info' };
		if (!e.air_date || e.air_date > today) return { label: 'Not aired', tone: 'neutral' };
		return e.monitored ? { label: 'Missing', tone: 'warning' } : { label: 'Unmonitored', tone: 'neutral' };
	}

	function downloadSeason(n: number) {
		const source: ReleaseSource =
			librarySeriesId !== undefined
				? { kind: 'season', seriesId: librarySeriesId, season: n }
				: { kind: 'discoverSeason', tmdbId: resolvedTMDBId!, season: n };
		ondownload({ title: `${title} · ${seasonLabel(n)}`, source });
	}
	function downloadEpisode(n: number, row: Row) {
		const source: ReleaseSource =
			librarySeriesId !== undefined && row.library
				? { kind: 'episode', seriesId: librarySeriesId, episodeId: row.library.id }
				: { kind: 'discoverEpisode', tmdbId: resolvedTMDBId!, season: n, episode: row.number };
		const name = row.meta?.title || row.library?.title;
		ondownload({ title: `${title} · ${episodeCode(n, row.number)}${name ? ` ${name}` : ''}`, source });
	}

	// Library-only controls: monitoring and automatic search.
	const setSeasonMonitored = useMutation('setSeasonMonitored');
	const searchSeason = useMutation('searchSeason');
	const setEpisodeMonitored = useMutation('setEpisodeMonitored');
	const searchEpisode = useMutation('searchEpisode');
	let searching = $state<string | null>(null);

	function autoSearchSeason(id: number, n: number) {
		searching = `s${n}`;
		searchSeason.mutate(
			{ id, season: n },
			{
				onSuccess: (results) => {
					const grabbed = results.filter((r) => r.grabbed).length;
					toasts.push(grabbed ? 'success' : 'info', grabbed ? `Grabbed ${grabbed} release(s)` : 'No acceptable releases found');
				},
				onSettled: () => (searching = null)
			}
		);
	}
	function autoSearchEpisode(id: number, e: Episode) {
		searching = `e${e.id}`;
		searchEpisode.mutate(
			{ episodeId: e.id, seriesId: id },
			{
				onSuccess: (r) =>
					toasts.push(r.grabbed ? 'success' : 'info', r.grabbed ? `Grabbed ${r.grabbed.release_title}` : 'No acceptable release found'),
				onSettled: () => (searching = null)
			}
		);
	}
</script>

{#if show.isPending && libSeries.isPending && seasons.length === 0}
	<Spinner label="Loading seasons" />
{:else if seasons.length === 0}
	{#if show.isError && !series.data}
		<ErrorMessage error={show.error} />
	{:else}
		<EmptyState message="TMDB lists no episodes for this show yet." />
	{/if}
{:else if season !== undefined && current}
	<section class="browser" aria-label="Seasons">
		<Tabs
			label="Seasons"
			tabs={seasons.map((s) => ({ key: String(s.number), label: s.number === 0 ? 'Specials' : `S${s.number}` }))}
			active={String(season)}
			onselect={(k) => (picked = Number(k))}
		/>

		<div class="season-head">
			<div class="season-title">
				<Heading level={2}>{current.name || seasonLabel(season)}</Heading>
				<Text size="sm" tone="muted" as="span">
					{current.episodes} episodes{current.airDate ? ` · ${current.airDate.slice(0, 4)}` : ''}
				</Text>
				{#if seasonStatus}
					<Badge tone={seasonStatus.complete ? 'success' : seasonStatus.downloaded > 0 ? 'info' : 'neutral'}>
						{seasonStatus.downloaded}/{seasonStatus.aired} downloaded
					</Badge>
				{/if}
			</div>
			<div class="season-actions">
				{#if librarySeriesId !== undefined && seasonStatus}
					<MonitorToggle
						monitored={seasonStatus.monitored > 0}
						what={season === 0 ? 'specials' : `season ${season}`}
						onchange={(v) => setSeasonMonitored.mutate({ seriesId: librarySeriesId, season, monitored: v })}
					/>
					<Button size="sm" loading={searching === `s${season}`} onclick={() => autoSearchSeason(librarySeriesId, season)}>
						<Icon name="search" size={14} />Search
					</Button>
				{/if}
				<Button size="sm" variant="primary" onclick={() => downloadSeason(season)}>
					<Icon name="download" size={14} />Download season
				</Button>
			</div>
		</div>
		{#if details.data?.overview}<Text size="sm" tone="muted">{details.data.overview}</Text>{/if}
		{#if details.isError}<ErrorMessage error={details.error} />{/if}

		{#if loadingEpisodes}
			<Spinner label="Loading episodes" />
		{:else}
			<div class="episodes">
				{#each rows as row (row.number)}
					{@const lib = row.library}
					<EpisodeCard
						season={season}
						episode={row.number}
						title={row.meta?.title || lib?.title || ''}
						overview={row.meta?.overview}
						airDate={row.meta?.air_date ?? lib?.air_date}
						runtime={row.meta?.runtime}
						rating={row.meta?.vote_average}
						stillSrc={stillURL(row.meta?.still_path)}
					>
						{#snippet status()}
							{#if lib}
								{@const st = episodeState(lib)}
								<Badge tone={st.tone}>{st.label}</Badge>
							{/if}
						{/snippet}
						{#snippet actions()}
							{#if librarySeriesId !== undefined && lib}
								<MonitorToggle
									monitored={lib.monitored}
									what={episodeCode(lib.season, lib.episode)}
									onchange={(v) => setEpisodeMonitored.mutate({ seriesId: librarySeriesId, episodeId: lib.id, monitored: v })}
								/>
								<Button size="sm" loading={searching === `e${lib.id}`} onclick={() => autoSearchEpisode(librarySeriesId, lib)}>
									<Icon name="search" size={14} />Search
								</Button>
							{/if}
							<Button size="sm" aria-label="Download {episodeCode(season, row.number)}" onclick={() => downloadEpisode(season, row)}>
								<Icon name="download" size={14} />Download
							</Button>
						{/snippet}
					</EpisodeCard>
				{/each}
			</div>
		{/if}
	</section>
{/if}

<style>
	.browser {
		display: grid;
		gap: var(--space-4);
		min-width: 0;
	}
	.season-head {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-3);
	}
	.season-title,
	.season-actions {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-3);
	}
	.episodes {
		display: grid;
		gap: var(--space-1);
	}
</style>
