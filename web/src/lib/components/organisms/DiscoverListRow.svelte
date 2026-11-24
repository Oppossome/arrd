<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { Heading, Skeleton } from '#lib/components/atoms/index.js';
	import { ErrorMessage, PosterCard, PosterRow } from '#lib/components/molecules/index.js';
	import { posterCardProps, type DiscoverList } from '#lib/domain/discover.js';
	import type { MediaKind } from '#lib/nav/links.js';
	import { queries } from '#lib/api/query.js';

	interface Props {
		kind: MediaKind;
		/** Which of TMDB's curated lists. */
		list: DiscoverList;
		title: string;
	}
	let { kind, list, title }: Props = $props();

	const items = createQuery(() => queries.discoverList(kind, list));
</script>

{#if items.isPending}
	<PosterRow {title}>{#each Array(8) as _, i (i)}<Skeleton height="16rem" />{/each}</PosterRow>
{:else if items.isError}
	<section class="failed" aria-label={title}>
		<Heading level={2}>{title}</Heading>
		<ErrorMessage error={items.error} />
	</section>
{:else if items.data.length > 0}
	<PosterRow {title}>
		{#each items.data as item (item.tmdb_id)}<PosterCard {...posterCardProps(kind, item)} />{/each}
	</PosterRow>
{/if}

<style>
	.failed {
		display: grid;
		gap: var(--space-3);
	}
</style>
