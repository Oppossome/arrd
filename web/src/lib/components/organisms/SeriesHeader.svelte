<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { goto } from '$app/navigation';
	import { Badge, Button, Heading, Poster, Skeleton, Text } from '#lib/components/atoms/index.js';
	import { ConfirmDialog, ErrorMessage, MonitorToggle } from '#lib/components/molecules/index.js';
	import { posterURL } from '#lib/api.js';
	import { LIBRARY } from '#lib/links.js';
	import { timeAgo } from '#lib/format.js';
	import { queries, useMutation } from '#lib/query.js';
	import { toasts } from '#lib/toast.svelte.js';

	let { id }: { id: number } = $props();

	const series = createQuery(() => queries.series(id));
	const monitor = useMutation('setSeriesMonitored');
	const refresh = useMutation('refreshSeries');
	const search = useMutation('searchSeriesWanted');
	const del = useMutation('deleteSeries');
	let confirming = $state(false);
</script>

{#if series.isPending}
	<Skeleton height="12rem" />
{:else if series.isError}
	<ErrorMessage error={series.error} />
{:else}
	{@const s = series.data}
	<div class="header">
		<div class="poster"><Poster src={posterURL(s.poster_path, 'w342')} alt={s.title} /></div>
		<div class="info">
			<div class="title">
				<Heading level={1}>{s.title}</Heading>
				<Text tone="muted" as="span">{s.year || ''}</Text>
				{#if s.status}<Badge>{s.status}</Badge>{/if}
			</div>
			<Text size="sm">{s.overview}</Text>
			<Text size="xs" tone="muted">Metadata refreshed {timeAgo(s.last_refreshed_at ?? s.added_at)}</Text>
			<div class="actions">
				<label class="monitor">
					<MonitorToggle monitored={s.monitored} what="this series" onchange={(v) => monitor.mutate({ id, monitored: v })} />
					<Text size="sm" as="span">{s.monitored ? 'Monitored' : 'Not monitored'}</Text>
				</label>
				<Button
					variant="primary"
					loading={search.isPending}
					onclick={() => search.mutate(id, { onSuccess: () => toasts.push('info', 'Search finished; see Activity for grabs') })}
					>Search wanted episodes</Button
				>
				<Button loading={refresh.isPending} onclick={() => refresh.mutate(id)}>Refresh metadata</Button>
				<Button variant="danger" onclick={() => (confirming = true)}>Delete</Button>
			</div>
		</div>
	</div>
	<ConfirmDialog
		open={confirming}
		title="Delete {s.title}?"
		confirmLabel="Delete"
		danger
		busy={del.isPending}
		onconfirm={() => del.mutate(id, { onSuccess: () => void goto(LIBRARY) })}
		oncancel={() => (confirming = false)}
	>
		<Text size="sm">The series and its episodes are removed from the library. Files on disk are not deleted.</Text>
	</ConfirmDialog>
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
	@media (max-width: 40rem) {
		.header {
			grid-template-columns: 1fr;
		}
		.poster {
			max-width: 10rem;
		}
	}
</style>
