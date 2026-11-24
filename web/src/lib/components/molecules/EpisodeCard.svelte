<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Rating, Text } from '#lib/components/atoms/index.js';
	import { episodeCode, formatDate, formatRuntime } from '#lib/utils/format.js';

	interface Props {
		season: number;
		episode: number;
		title: string;
		overview?: string;
		airDate?: string;
		/** Minutes. */
		runtime?: number;
		rating?: number;
		stillSrc?: string;
		/** Status badge (library episodes). */
		status?: Snippet;
		/** Buttons for this episode. */
		actions?: Snippet;
	}
	let { season, episode, title, overview, airDate, runtime, rating, stillSrc, status, actions }: Props = $props();
	let failed = $state(false);
	const code = $derived(episodeCode(season, episode));
</script>

<article class="episode" aria-label="{code} {title}">
	<div class="still">
		{#if stillSrc && !failed}
			<img src={stillSrc} alt="" loading="lazy" onerror={() => (failed = true)} />
		{:else}
			<span aria-hidden="true">{episode}</span>
		{/if}
	</div>
	<div class="info">
		<div class="head">
			<Text size="sm" mono as="span">{code}</Text>
			<span class="name"><Text as="span">{title || 'TBA'}</Text></span>
			{@render status?.()}
		</div>
		<div class="meta">
			<Text size="xs" tone="muted" as="span">{formatDate(airDate)}</Text>
			{#if runtime}<Text size="xs" tone="muted" as="span">{formatRuntime(runtime)}</Text>{/if}
			{#if rating}<Rating value={rating} />{/if}
		</div>
		{#if overview}<div class="overview"><Text size="sm" tone="muted">{overview}</Text></div>{/if}
	</div>
	{#if actions}<div class="actions">{@render actions()}</div>{/if}
</article>

<style>
	.episode {
		display: grid;
		grid-template-columns: 10rem 1fr auto;
		gap: var(--space-4);
		align-items: center;
		padding: var(--space-3);
		border-radius: var(--radius-md);
		transition: background var(--transition-fast);
	}
	.episode:hover {
		background: var(--color-surface-raised);
	}
	.still {
		display: grid;
		place-items: center;
		aspect-ratio: 16 / 9;
		border-radius: var(--radius-sm);
		overflow: hidden;
		background: var(--color-surface-raised);
		color: var(--color-text-muted);
		font-size: var(--text-xl);
		font-weight: var(--weight-bold);
	}
	img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		display: block;
	}
	.info {
		display: grid;
		gap: var(--space-1);
		min-width: 0;
	}
	.head,
	.meta {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-2) var(--space-3);
	}
	.name {
		font-weight: var(--weight-medium);
	}
	.overview {
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}
	.actions {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: flex-end;
		gap: var(--space-2);
	}
	@media (max-width: 40rem) {
		.episode {
			grid-template-columns: 7rem 1fr;
		}
		.actions {
			grid-column: 1 / -1;
			justify-content: flex-start;
		}
	}
</style>
