<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { ErrorMessage, PosterCard, PosterRow } from '#lib/components/molecules/index.js';
	import { ApiError } from '#lib/api/client.js';
	import { posterCardProps } from '#lib/domain/discover.js';
	import type { MediaKind } from '#lib/nav/links.js';
	import { queries } from '#lib/api/query.js';

	interface Props {
		kind: MediaKind;
		tmdbId: number;
	}
	let { kind, tmdbId }: Props = $props();

	const like = createQuery(() => queries.like(kind, tmdbId));
	// The page's header already says when TMDB doesn't know the title.
	const notFound = $derived(like.error instanceof ApiError && like.error.status === 404);
</script>

{#if like.isError && !notFound}
	<ErrorMessage error={like.error} />
{:else if like.data && like.data.length > 0}
	<PosterRow title="More like this">
		{#each like.data as item (item.tmdb_id)}<PosterCard {...posterCardProps(kind, item)} />{/each}
	</PosterRow>
{/if}
