<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { Button, Icon, Link, Text } from '#lib/components/atoms/index.js';
	import { ErrorMessage } from '#lib/components/molecules/index.js';
	import { libraryHref } from '#lib/nav/links.js';
	import { queries, useMutation } from '#lib/api/query.js';
	import type { SourcesRequest } from '#lib/domain/releases.js';
	import { toasts } from '#lib/stores/toast.svelte.js';

	interface Props {
		kind: 'movies' | 'series';
		tmdbId: number;
		/** Show the sources to download from: the movie, or whole-series packs of a series. */
		ondownload?: (request: SourcesRequest) => void;
	}
	let { kind, tmdbId, ondownload }: Props = $props();

	const movie = createQuery(() => ({ ...queries.discoverMovie(tmdbId), enabled: kind === 'movies' }));
	const show = createQuery(() => ({ ...queries.discoverShow(tmdbId), enabled: kind === 'series' }));
	const item = $derived(kind === 'movies' ? movie.data : show.data);
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

{#if item}
	{#if kind === 'movies' && ondownload}
		<Button variant="primary" onclick={() => ondownload({ title: item.title, source: { kind: 'discoverMovie', tmdbId } })}>
			<Icon name="download" />Download
		</Button>
	{:else if kind === 'series' && ondownload}
		<Button
			variant="primary"
			onclick={() => ondownload({ title: `${item.title} · Whole series`, source: { kind: 'discoverSeries', tmdbId } })}
		>
			<Icon name="download" />Download series
		</Button>
	{/if}
	{#if inLibrary}
		<Text size="sm"><Link href={inLibrary}>Open in library</Link> to see downloads and files.</Text>
	{:else}
		<Button loading={adding} onclick={addToLibrary}><Icon name="plus" />Add to library (monitored)</Button>
	{/if}
	{#if addError}<ErrorMessage error={addError} />{/if}
{/if}
