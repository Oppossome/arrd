<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { goto } from '$app/navigation';
	import { Badge, Button, Icon, IconButton, Text } from '#lib/components/atoms/index.js';
	import { ConfirmDialog, ErrorMessage, MonitorToggle } from '#lib/components/molecules/index.js';
	import { LIBRARY } from '#lib/nav/links.js';
	import { timeAgo } from '#lib/utils/format.js';
	import { queries, useMutation } from '#lib/api/query.js';
	import { toasts } from '#lib/stores/toast.svelte.js';

	let { id }: { id: number } = $props();

	const series = createQuery(() => queries.series(id));
	const monitor = useMutation('setSeriesMonitored');
	const refresh = useMutation('refreshSeries');
	const search = useMutation('searchSeriesWanted');
	const del = useMutation('deleteSeries');
	let confirming = $state(false);
</script>

{#if series.isError}
	<ErrorMessage error={series.error} />
{:else if series.data}
	{@const s = series.data}
	<div class="stack">
		<div class="actions">
			<Button
				variant="primary"
				loading={search.isPending}
				onclick={() => search.mutate(id, { onSuccess: () => toasts.push('info', 'Search finished; see Activity for grabs') })}
				><Icon name="search" />Search wanted episodes</Button
			>
			<MonitorToggle
				showLabel
				monitored={s.monitored}
				what="this series"
				disabled={monitor.isPending}
				onchange={(v) => monitor.mutate({ id, monitored: v })}
			/>
			<span class="end">
				<IconButton
					icon="refresh"
					label="Refresh metadata"
					spinning={refresh.isPending}
					onclick={() => refresh.mutate(id)}
				/>
				<IconButton icon="trash" tone="danger" label="Delete series" onclick={() => (confirming = true)} />
			</span>
		</div>
		<div class="meta">
			{#if s.status}<Badge>{s.status}</Badge>{/if}
			<Text size="xs" tone="muted" as="span">Metadata refreshed {timeAgo(s.last_refreshed_at ?? s.added_at)}</Text>
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
		display: flex;
		gap: var(--space-1);
		margin-left: auto;
	}
	.meta {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-2);
	}
</style>
