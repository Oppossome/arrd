<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { goto } from '$app/navigation';
	import { Button, Heading, Poster, Skeleton, Text } from '#lib/components/atoms/index.js';
	import { ConfirmDialog, ErrorMessage, MonitorToggle, StatusBadge } from '#lib/components/molecules/index.js';
	import { posterURL } from '#lib/api.js';
	import { LIBRARY } from '#lib/links.js';
	import { queries, useMutation } from '#lib/query.js';
	import { toasts } from '#lib/toast.svelte.js';

	let { id }: { id: number } = $props();

	const movie = createQuery(() => queries.movie(id));
	const monitor = useMutation('setMovieMonitored');
	const search = useMutation('searchMovie');
	const del = useMutation('deleteMovie');
	let confirming = $state(false);

	function autoSearch() {
		search.mutate(id, {
			onSuccess: (r) =>
				r.grabbed
					? toasts.push('success', `Grabbed ${r.grabbed.release_title}`)
					: toasts.push('info', `No acceptable release among ${r.releases_considered} results`)
		});
	}
</script>

{#if movie.isPending}
	<Skeleton height="12rem" />
{:else if movie.isError}
	<ErrorMessage error={movie.error} />
{:else}
	{@const m = movie.data}
	<div class="header">
		<div class="poster"><Poster src={posterURL(m.poster_path, 'w342')} alt={m.title} /></div>
		<div class="info">
			<div class="title">
				<Heading level={1}>{m.title}</Heading>
				<Text tone="muted" as="span">{m.year || 'Year unknown'}</Text>
				<StatusBadge status={m.status} />
			</div>
			<Text size="sm">{m.overview}</Text>
			{#if m.file_path}
				<Text size="sm" tone="success">{m.file_quality} · <span class="path">{m.file_path}</span></Text>
			{/if}
			<div class="actions">
				<label class="monitor">
					<MonitorToggle monitored={m.monitored} what="this movie" onchange={(v) => monitor.mutate({ id, monitored: v })} />
					<Text size="sm" as="span">{m.monitored ? 'Monitored' : 'Not monitored'}</Text>
				</label>
				<Button variant="primary" loading={search.isPending} onclick={autoSearch}>Automatic search</Button>
				<Button variant="danger" onclick={() => (confirming = true)}>Delete</Button>
			</div>
		</div>
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
	.header {
		display: grid;
		grid-template-columns: 12rem 1fr;
		gap: var(--space-5);
	}
	.info,
	.title {
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
	.actions {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-3);
	}
	.monitor {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}
	.path {
		font-family: var(--font-mono);
		word-break: break-all;
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
