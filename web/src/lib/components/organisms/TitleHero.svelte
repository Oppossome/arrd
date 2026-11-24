<script lang="ts">
	import type { Snippet } from 'svelte';
	import { createQuery } from '@tanstack/svelte-query';
	import { Heading, Skeleton, Text } from '#lib/components/atoms/index.js';
	import { CastRow, ErrorMessage, MediaHero } from '#lib/components/molecules/index.js';
	import { backdropURL, profileURL } from '#lib/api.js';
	import { queries } from '#lib/query.js';
	import type { MediaExtras } from '#lib/types.js';

	interface Props {
		kind: 'movies' | 'series';
		/** The TMDB title to show... */
		tmdbId?: number;
		/** ...or a library title, whose TMDB details are looked up. */
		libraryId?: number;
		/** Controls under the overview (an actions organism the page supplies). */
		actions?: Snippet;
	}
	let { kind, tmdbId, libraryId, actions }: Props = $props();

	const isMovie = $derived(kind === 'movies');
	const libMovie = createQuery(() => ({ ...queries.movie(libraryId ?? 0), enabled: isMovie && libraryId !== undefined }));
	const libSeries = createQuery(() => ({ ...queries.series(libraryId ?? 0), enabled: !isMovie && libraryId !== undefined }));
	const library = $derived(isMovie ? libMovie : libSeries);

	const resolvedTMDBId = $derived(tmdbId ?? library.data?.tmdb_id);
	const movie = createQuery(() => ({ ...queries.discoverMovie(resolvedTMDBId ?? 0), enabled: isMovie && !!resolvedTMDBId }));
	const show = createQuery(() => ({ ...queries.discoverShow(resolvedTMDBId ?? 0), enabled: !isMovie && !!resolvedTMDBId }));
	const tmdb = $derived(isMovie ? movie : show);

	// TMDB has the richest details; a library title still shows without it.
	const item = $derived(tmdb.data ?? library.data);
	const extras = $derived<MediaExtras | undefined>(tmdb.data?.extras);
	const pending = $derived(!item && (library.isPending || tmdb.isPending) && !library.isError && !tmdb.isError);
	const error = $derived(!item ? (library.error ?? tmdb.error) : null);
</script>

{#if pending}
	<Skeleton height="28rem" />
{:else if error}
	<ErrorMessage {error} />
{:else if item}
	<MediaHero
		title={item.title}
		year={item.year}
		overview={item.overview}
		tagline={extras?.tagline}
		backdropSrc={backdropURL(item.backdrop_path, 'w1280')}
		trailerKey={extras?.trailer?.youtube_key}
		rating={extras?.vote_average}
		votes={extras?.vote_count}
		certification={extras?.certification}
		runtime={extras?.runtime}
		genres={extras?.genres.map((g) => g.name)}
		networks={extras?.networks}
		{actions}
	/>
	{#if extras && extras.cast.length > 0}
		<section class="cast" aria-label="Cast">
			<Heading level={2}>Cast</Heading>
			<CastRow
				cast={extras.cast.map((c) => ({ name: c.name, character: c.character, photoSrc: profileURL(c.profile_path) }))}
			/>
		</section>
	{:else if tmdb.isError}
		<div class="cast"><Text size="xs" tone="muted">TMDB details are unavailable right now.</Text></div>
	{/if}
{/if}

<style>
	.cast {
		display: grid;
		gap: var(--space-3);
		padding: var(--space-5) var(--space-5) 0;
	}
</style>
