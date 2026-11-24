<script lang="ts">
	import { Badge, Button, Text } from '#lib/components/atoms/index.js';
	import { formatBytes, timeAgo } from '#lib/utils/format.js';
	import { rejectionSummary, releaseCoverage } from '#lib/domain/releases.js';
	import type { ReleaseResult } from '#lib/api/types.js';

	interface Props {
		release: ReleaseResult;
		/** A grab of this release is under way. */
		loading?: boolean;
		ongrab: () => void;
	}
	let { release: r, loading = false, ongrab }: Props = $props();

	const coverage = $derived(releaseCoverage(r));
	const why = $derived(rejectionSummary(r));
</script>

<li class="row" class:rejected={!r.accepted}>
	<div class="main">
		<div class="title">
			<Text size="sm" as="span">{r.title}</Text>
			{#if coverage}<Badge tone="info">{coverage}</Badge>{/if}
		</div>
		<div class="meta">
			<Badge tone={r.accepted ? 'success' : 'neutral'}>{r.quality}</Badge>
			<span>{formatBytes(r.size)}</span>
			<span title="Seeders / peers">↑{r.seeders} <span class="peers">/ {r.peers}</span></span>
			<span>{r.indexer}</span>
			<span>{timeAgo(r.publish_date)}</span>
		</div>
		{#if !r.accepted && why}
			<p class="why" title={why}><Text size="xs" tone="danger" as="span">{why}</Text></p>
		{/if}
	</div>
	<div class="action">
		<Button size="sm" variant={r.accepted ? 'primary' : 'ghost'} {loading} onclick={ongrab}>
			{r.accepted ? 'Grab' : 'Grab anyway'}
		</Button>
	</div>
</li>

<style>
	.row {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		align-items: center;
		gap: var(--space-4);
		padding: var(--space-3) var(--space-2);
		border-bottom: 1px solid var(--color-border);
	}
	.main {
		display: grid;
		gap: var(--space-1);
		min-width: 0;
	}
	.title {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-2);
		overflow-wrap: anywhere;
	}
	.meta {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-1) var(--space-3);
		color: var(--color-text-muted);
		font-size: var(--text-xs);
		font-variant-numeric: tabular-nums;
	}
	.peers {
		opacity: 0.7;
	}
	/* One line of reasons, two at most when it wraps; the rest on hover. */
	.why {
		margin: 0;
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}
	.rejected .title,
	.rejected .meta {
		opacity: 0.65;
	}
	.rejected:hover .title,
	.rejected:hover .meta {
		opacity: 1;
	}
</style>
