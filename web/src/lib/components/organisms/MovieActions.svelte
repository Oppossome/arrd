<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { goto } from '$app/navigation';
	import { Button, Icon, IconButton, Text } from '#lib/components/atoms/index.js';
	import { ConfirmDialog, ErrorMessage, MonitorToggle, StatusBadge } from '#lib/components/molecules/index.js';
	import { LIBRARY } from '#lib/nav/links.js';
	import { queries, useMutation } from '#lib/api/query.js';
	import type { SourcesRequest } from '#lib/domain/releases.js';
	import { toasts } from '#lib/stores/toast.svelte.js';

	interface Props {
		id: number;
		/** Show the sources to download from. */
		ondownload: (request: SourcesRequest) => void;
	}
	let { id, ondownload }: Props = $props();

	const movie = createQuery(() => queries.movie(id));
	// TMDB's view of the movie says which collection, if any, it belongs to.
	const tmdbMovie = createQuery(() => ({ ...queries.discoverMovie(movie.data?.tmdb_id ?? 0), enabled: !!movie.data }));
	const collection = $derived(tmdbMovie.data?.collection);
	const monitor = useMutation('setMovieMonitored');
	const search = useMutation('autoSearch');
	const del = useMutation('deleteMovie');
	let confirming = $state(false);

	function autoSearch(tmdbId: number) {
		search.mutate({ kind: 'movie', tmdb_id: tmdbId }, {
			onSuccess: (r) =>
				r.grabbed.length > 0
					? toasts.push('success', `Grabbed ${r.grabbed[0].release_title}`)
					: toasts.push('info', `No acceptable release among ${r.releases_considered} results`)
		});
	}
</script>

{#if movie.isError}
	<ErrorMessage error={movie.error} />
{:else if movie.data}
	{@const m = movie.data}
	{@const showStatus = m.status !== 'unmonitored'}
	<div class="stack">
		<div class="actions">
			<Button variant="primary" onclick={() => ondownload({ title: m.title, target: { kind: 'movie', tmdb_id: m.tmdb_id } })}>
				<Icon name="download" />Download
			</Button>
			{#if collection}
				<Button
					onclick={() =>
						ondownload({ title: `${collection.name} · Movie packs`, target: { kind: 'collection', tmdb_id: m.tmdb_id } })}
				>
					<Icon name="download" />Download collection
				</Button>
			{/if}
			<Button loading={search.isPending} onclick={() => autoSearch(m.tmdb_id)}><Icon name="search" />Automatic search</Button>
			<MonitorToggle
				showLabel
				monitored={m.monitored}
				what="this movie"
				disabled={monitor.isPending}
				onchange={(v) => monitor.mutate({ id, monitored: v })}
			/>
			<span class="end">
				<IconButton icon="trash" tone="danger" label="Delete movie" onclick={() => (confirming = true)} />
			</span>
		</div>
		{#if showStatus || m.file_path}
			<div class="meta">
				{#if showStatus}<StatusBadge status={m.status} />{/if}
				{#if m.file_path}
					<Text size="sm" tone="success" as="span">{m.file_quality} · <span class="path">{m.file_path}</span></Text>
				{/if}
			</div>
		{/if}
	</div>
	<ConfirmDialog
		open={confirming}
		title="Delete {m.title}?"
		confirmLabel="Delete"
		danger
		busy={del.isPending}
		onconfirm={() => del.mutate(id, { onSuccess: () => void goto(LIBRARY) })}
		oncancel={() => (confirming = false)}
	>
		<Text size="sm">The movie is removed from the library. Files on disk are not deleted.</Text>
	</ConfirmDialog>
{/if}

<style>
	.stack {
		display: grid;
		gap: var(--space-3);
	}
	.actions {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-3);
	}
	.end {
		margin-left: auto;
	}
	.meta {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-2);
	}
	.path {
		font-family: var(--font-mono);
		word-break: break-all;
	}
</style>
