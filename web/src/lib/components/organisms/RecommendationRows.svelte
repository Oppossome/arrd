<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { ErrorMessage, PosterCard, PosterRow } from '#lib/components/molecules/index.js';
	import { posterCardProps } from '#lib/domain/discover.js';
	import type { MediaKind } from '#lib/nav/links.js';
	import { queries } from '#lib/api/query.js';

	/** "Because you added X" rows, one per recently added library title. */
	let { kind }: { kind: MediaKind } = $props();

	const rows = createQuery(() => queries.recommendations(kind));
</script>

{#if rows.isError}
	<ErrorMessage error={rows.error} />
{:else}
	{#each rows.data ?? [] as row (row.because.tmdb_id)}
		<PosterRow title="Because you added {row.because.title}">
			{#each row.items as item (item.tmdb_id)}<PosterCard {...posterCardProps(kind, item)} />{/each}
		</PosterRow>
	{/each}
{/if}
