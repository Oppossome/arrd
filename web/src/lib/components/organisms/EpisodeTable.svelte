<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { Badge, Button, Spinner, Text } from '#lib/components/atoms/index.js';
	import { ErrorMessage, MonitorToggle } from '#lib/components/molecules/index.js';
	import { episodeCode, formatDate } from '#lib/format.js';
	import { queries, useMutation } from '#lib/query.js';
	import { toasts } from '#lib/toast.svelte.js';
	import type { Episode } from '#lib/types.js';

	interface Props {
		seriesId: number;
		season: number;
		/** Called when the user wants to see releases for one episode. */
		onreleases?: (episode: Episode) => void;
	}
	let { seriesId, season, onreleases }: Props = $props();

	const episodes = createQuery(() => queries.episodes(seriesId));
	const setMonitored = useMutation('setEpisodeMonitored');
	const search = useMutation('searchEpisode');
	let searching = $state<number | null>(null);

	const rows = $derived((episodes.data ?? []).filter((e) => e.season === season).sort((a, b) => b.episode - a.episode));
	const today = new Date().toISOString().slice(0, 10);

	function episodeState(e: Episode): { label: string; tone: 'success' | 'info' | 'warning' | 'neutral' } {
		if (e.file_path) return { label: e.file_quality ?? 'Downloaded', tone: 'success' };
		if (e.downloading) return { label: 'Downloading', tone: 'info' };
		if (!e.air_date || e.air_date > today) return { label: 'Not aired', tone: 'neutral' };
		return e.monitored ? { label: 'Missing', tone: 'warning' } : { label: 'Unmonitored', tone: 'neutral' };
	}

	function autoSearch(e: Episode) {
		searching = e.id;
		search.mutate(
			{ episodeId: e.id, seriesId },
			{
				onSuccess: (r) =>
					toasts.push(r.grabbed ? 'success' : 'info', r.grabbed ? `Grabbed ${r.grabbed.release_title}` : 'No acceptable release found'),
				onSettled: () => (searching = null)
			}
		);
	}
</script>

{#if episodes.isPending}
	<Spinner label="Loading episodes" />
{:else if episodes.isError}
	<ErrorMessage error={episodes.error} />
{:else}
	<div class="table-wrap">
		<table>
			<thead>
				<tr>
					<th><span class="visually-hidden">Monitored</span></th>
					<th>#</th>
					<th>Title</th>
					<th>Air date</th>
					<th>Status</th>
					<th><span class="visually-hidden">Actions</span></th>
				</tr>
			</thead>
			<tbody>
				{#each rows as e (e.id)}
					{@const st = episodeState(e)}
					<tr>
						<td>
							<MonitorToggle
								monitored={e.monitored}
								what={episodeCode(e.season, e.episode)}
								onchange={(v) => setMonitored.mutate({ seriesId, episodeId: e.id, monitored: v })}
							/>
						</td>
						<td><Text size="sm" mono as="span">{episodeCode(e.season, e.episode)}</Text></td>
						<td><Text size="sm" as="span">{e.title || 'TBA'}</Text></td>
						<td><Text size="sm" tone="muted" as="span">{formatDate(e.air_date)}</Text></td>
						<td><Badge tone={st.tone}>{st.label}</Badge></td>
						<td class="actions">
							<Button size="sm" loading={searching === e.id} onclick={() => autoSearch(e)}>Search</Button>
							{#if onreleases}<Button size="sm" variant="ghost" onclick={() => onreleases(e)}>Releases</Button>{/if}
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}

<style>
	.table-wrap {
		overflow-x: auto;
	}
	table {
		width: 100%;
		border-collapse: collapse;
	}
	th,
	td {
		padding: var(--space-2);
		border-bottom: 1px solid var(--color-border);
		text-align: left;
		vertical-align: middle;
	}
	th {
		color: var(--color-text-muted);
		font-size: var(--text-xs);
		font-weight: var(--weight-medium);
	}
	.actions {
		display: flex;
		gap: var(--space-1);
		justify-content: flex-end;
	}
</style>
