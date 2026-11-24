<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { Badge, Button, Heading, Link, Poster, Select, Skeleton, Text } from '#lib/components/atoms/index.js';
	import { ErrorMessage } from '#lib/components/molecules/index.js';
	import { posterURL } from '#lib/api.js';
	import { libraryHref } from '#lib/links.js';
	import { queries, useMutation } from '#lib/query.js';
	import { toasts } from '#lib/toast.svelte.js';
	import type { LookupMovie, LookupSeries } from '#lib/types.js';

	interface Props {
		kind: 'movies' | 'series';
		tmdbId: number;
		/** For series: the season to show torrents for. Starts at the latest season. */
		season?: number;
	}
	let { kind, tmdbId, season = $bindable() }: Props = $props();

	const movie = createQuery(() => ({ ...queries.discoverMovie(tmdbId), enabled: kind === 'movies' }));
	const show = createQuery(() => ({ ...queries.discoverShow(tmdbId), enabled: kind === 'series' }));
	const q = $derived(kind === 'movies' ? movie : show);
	const item = $derived<LookupMovie | LookupSeries | undefined>(kind === 'movies' ? movie.data : show.data);

	// Seasons with episodes, newest first; specials (season 0) last.
	const seasons = $derived(
		(show.data?.seasons ?? [])
			.filter((s) => s.episode_count > 0)
			.toSorted((a, b) => (a.season_number === 0 ? 1 : b.season_number === 0 ? -1 : b.season_number - a.season_number))
	);
	$effect(() => {
		if (season === undefined && seasons.length > 0) season = seasons[0].season_number;
	});
	const inLibrary = $derived(item?.library_id ? libraryHref(kind, item.library_id) : undefined);

	// Monitored titles are downloaded automatically when a good release appears.
	const addMovie = useMutation('addMovie');
	const addSeries = useMutation('addSeries');
	const adding = $derived(addMovie.isPending || addSeries.isPending);
	const addError = $derived(addMovie.error ?? addSeries.error);
	function addToLibrary() {
		const done = { onSuccess: () => toasts.push('success', 'Added to your library; arrd will download it when a good release appears') };
		if (kind === 'movies') addMovie.mutate({ tmdb_id: tmdbId, monitored: true, search: true }, done);
		else addSeries.mutate({ tmdb_id: tmdbId, monitor: 'all', search: true }, done);
	}
</script>

{#if q.isPending}
	<Skeleton height="12rem" />
{:else if q.isError}
	<ErrorMessage error={q.error} />
{:else if item}
	<div class="header">
		<div class="poster"><Poster src={posterURL(item.poster_path, 'w342')} alt={item.title} /></div>
		<div class="info">
			<div class="title">
				<Heading level={1}>{item.title}</Heading>
				<Text tone="muted" as="span">{item.year || 'Year unknown'}</Text>
				{#if inLibrary}<Badge tone="info">In library</Badge>{/if}
			</div>
			<Text size="sm">{item.overview}</Text>
			{#if inLibrary}
				<Text size="sm"><Link href={inLibrary}>Open in library</Link> to see downloads and files.</Text>
			{:else}
				<div class="add">
					<Button loading={adding} onclick={addToLibrary}>Add to library (monitored)</Button>
					<Text size="xs" tone="muted">Or pick a source below. Monitored titles are downloaded automatically when a good release appears.</Text>
				</div>
				{#if addError}<ErrorMessage error={addError} />{/if}
			{/if}
			{#if kind === 'series'}
				{#if seasons.length === 0}
					<Text size="sm" tone="muted">TMDB lists no episodes for this show yet.</Text>
				{:else if season !== undefined}
					<label class="season">
						<Text as="span" size="sm">Season</Text>
						<Select
							bind:value={season}
							options={seasons.map((s) => ({
								value: s.season_number,
								label: `${s.season_number === 0 ? 'Specials' : `Season ${s.season_number}`} (${s.episode_count} episodes)`
							}))}
						/>
					</label>
				{/if}
			{/if}
		</div>
	</div>
{/if}

<style>
	.header {
		display: grid;
		grid-template-columns: 12rem 1fr;
		gap: var(--space-5);
	}
	.info {
		display: grid;
		gap: var(--space-3);
		align-content: start;
	}
	.title {
		display: flex;
		flex-wrap: wrap;
		align-items: baseline;
		gap: var(--space-3);
	}
	.add {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-3);
	}
	.season {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		max-width: 20rem;
	}
	@media (max-width: 40rem) {
		.header {
			grid-template-columns: 1fr;
		}
		.poster {
			max-width: 10rem;
		}
	}
</style>
