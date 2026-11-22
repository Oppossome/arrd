<script lang="ts">
	import { Badge, Poster, ProgressRing, Text } from '#lib/components/atoms/index.js';
	import StatusBadge from './StatusBadge.svelte';
	import type { ItemDownload } from '#lib/downloads.js';
	import type { MovieStatus } from '#lib/types.js';

	interface Props {
		href: string;
		title: string;
		year?: number;
		posterSrc?: string;
		status?: MovieStatus;
		subtitle?: string;
		/** Shown over the poster while the item downloads or after a failure. */
		download?: ItemDownload;
	}
	let { href, title, year, posterSrc, status, subtitle, download }: Props = $props();
</script>

<a class="card" {href}>
	<div class="art">
		<Poster src={posterSrc} alt={title} />
		{#if download}
			<div class="overlay">
				{#if download.kind === 'failed'}
					<Badge tone="danger">Failed</Badge>
				{:else if download.importing}
					<Badge tone="info">Importing</Badge>
				{:else}
					<ProgressRing
						value={download.percent}
						label={download.paused ? 'Paused' : 'Download progress'}
						tone={download.paused ? 'neutral' : 'info'}
					/>
					{#if download.paused}<Badge tone="neutral">Paused</Badge>{/if}
				{/if}
			</div>
		{/if}
	</div>
	<div class="meta">
		<Text size="sm">{title}</Text>
		<div class="row">
			<Text size="xs" tone="muted" as="span">{year || '—'}{subtitle ? ` · ${subtitle}` : ''}</Text>
			{#if status}<StatusBadge {status} />{/if}
		</div>
	</div>
</a>

<style>
	.card {
		display: grid;
		gap: var(--space-2);
		color: inherit;
		text-decoration: none;
	}
	.card:hover {
		opacity: 0.85;
	}
	.art {
		position: relative;
	}
	.overlay {
		position: absolute;
		right: var(--space-2);
		bottom: var(--space-2);
		display: flex;
		align-items: center;
		gap: var(--space-1);
		padding: var(--space-1);
		border-radius: var(--radius-pill);
		background: var(--color-overlay);
	}
	.meta {
		display: grid;
		gap: var(--space-1);
	}
	.row {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: var(--space-2);
	}
</style>
