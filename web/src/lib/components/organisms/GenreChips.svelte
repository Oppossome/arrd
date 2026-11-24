<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { Chip, Text } from '#lib/components/atoms/index.js';
	import { ErrorMessage } from '#lib/components/molecules/index.js';
	import { browseHref, type MediaKind } from '#lib/links.js';
	import { queries } from '#lib/query.js';

	/** Links to browse TMDB's catalogue by genre. */
	let { kind }: { kind: MediaKind } = $props();

	const genres = createQuery(() => queries.genres(kind));
</script>

{#if genres.isError}
	<ErrorMessage error={genres.error} />
{:else if genres.data}
	<nav class="genres" aria-label="Browse by genre">
		<Text size="sm" tone="muted" as="span">Browse</Text>
		{#each genres.data as genre (genre.id)}
			<Chip href={browseHref(kind, { genres: [genre.id] })}>{genre.name}</Chip>
		{/each}
	</nav>
{/if}

<style>
	.genres {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-2);
	}
</style>
