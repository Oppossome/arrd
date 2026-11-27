<script lang="ts">
	import { IconButton, Link, ProgressBar, Text } from '#lib/components/atoms/index.js';
	import PeerSummary from './PeerSummary.svelte';
	import { backdropURL, posterURL } from '#lib/api/client.js';
	import { formatBytes, formatETA, formatRate, percent } from '#lib/utils/format.js';
	import { downloadContents } from '#lib/domain/downloads.js';
	import type { Download } from '#lib/api/types.js';

	interface Props {
		download: Download;
		/** The item's library page. */
		href?: string;
		busy?: boolean;
		onpause: () => void;
		onresume: () => void;
		onremove: () => void;
		onpeers: () => void;
	}
	let { download: d, href, busy = false, onpause, onresume, onremove, onpeers }: Props = $props();

	const name = $derived(d.title || d.release_title);
	const art = $derived(backdropURL(d.backdrop_path) ?? posterURL(d.poster_path, 'w342'));
	const blurred = $derived(!d.backdrop_path && !!d.poster_path);
	const p = $derived(d.progress);
	const pct = $derived(d.status === 'completed' ? 100 : p ? percent(p.bytes_completed, p.bytes_total) : 0);
	const state = $derived.by(() => {
		if (d.status === 'completed') return 'Importing';
		if (d.paused) return 'Paused';
		if (!p) return 'Not running';
		if (!p.have_info) return 'Fetching metadata…';
		return `${formatRate(p.download_rate)} · ${formatETA(p.eta_seconds)} left`;
	});
	const episodes = $derived(downloadContents(d));
</script>

<article class="card" aria-label={name}>
	{#if art}<img class="art" class:blurred src={art} alt="" loading="lazy" />{/if}
	<div class="shade"></div>
	<div class="content">
		<header>
			<div class="title">
				{#if href}<Link {href}>{name}</Link>{:else}<Text>{name}</Text>{/if}
				{#if d.year || episodes}
					<Text size="xs" tone="muted" as="span">{[d.year, episodes].filter(Boolean).join(' · ')}</Text>
				{/if}
			</div>
			<div class="actions">
				{#if d.status === 'downloading'}
					{#if d.paused}
						<IconButton icon="play" label="Resume" disabled={busy} onclick={onresume} />
					{:else}
						<IconButton icon="pause" label="Pause" disabled={busy} onclick={onpause} />
					{/if}
				{/if}
				<IconButton icon="trash" label="Remove" tone="danger" disabled={busy} onclick={onremove} />
			</div>
		</header>
		<div class="progress">
			<div class="line">
				<Text size="xs" as="span">{state}</Text>
				<Text size="xs" tone="muted" as="span">{pct}%</Text>
			</div>
			<ProgressBar
				value={pct}
				label="Download progress"
				tone={d.status === 'completed' ? 'success' : 'info'}
			/>
		</div>
		<footer>
			<Text size="xs" tone="muted" as="span">
				{#if p?.have_info}{formatBytes(p.bytes_completed)} / {formatBytes(p.bytes_total)}{:else}{formatBytes(d.size)}{/if}
				· {d.quality}
				{#if p && p.upload_rate > 0}· ↑ {formatRate(p.upload_rate)}{/if}
			</Text>
			{#if p}<PeerSummary countries={p.countries} onclick={onpeers} />{/if}
		</footer>
	</div>
</article>

<style>
	.card {
		position: relative;
		display: grid;
		min-height: 11rem;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-lg);
		overflow: hidden;
		background: var(--color-surface);
	}
	.art {
		position: absolute;
		inset: 0;
		width: 100%;
		height: 100%;
		object-fit: cover;
	}
	.art.blurred {
		filter: blur(1.5rem);
		transform: scale(1.2);
	}
	/* Fades the art into the card so text stays readable. */
	.shade {
		position: absolute;
		inset: 0;
		background: linear-gradient(to top, var(--color-surface) 35%, transparent),
			linear-gradient(to right, var(--color-overlay), transparent);
	}
	.content {
		position: relative;
		display: grid;
		align-content: space-between;
		gap: var(--space-3);
		padding: var(--space-4);
	}
	header,
	footer,
	.line {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: var(--space-2);
	}
	header {
		align-items: start;
	}
	.title {
		display: grid;
		gap: var(--space-1);
		min-width: 0;
		font-weight: var(--weight-medium);
	}
	.actions {
		display: flex;
		gap: var(--space-1);
		border-radius: var(--radius-md);
		background: var(--color-overlay);
	}
	.progress {
		display: grid;
		gap: var(--space-1);
	}
	footer {
		flex-wrap: wrap;
	}
</style>
