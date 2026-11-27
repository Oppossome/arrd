<script lang="ts">
	import { page } from '$app/state';
	import { DiscoverActions, MoreLikeThis, ReleaseTable, TitleHero } from '#lib/components/organisms/index.js';
	import { DetailLayout, SourcesDialog } from '#lib/components/templates/index.js';
	import type { SourcesRequest } from '#lib/domain/releases.js';

	const tmdbId = $derived(Number(page.params.tmdbId));
	let sources = $state<SourcesRequest | null>(null);
</script>

{#key tmdbId}
	<DetailLayout bleed>
		{#snippet header()}
			<TitleHero kind="movies" {tmdbId}>
				{#snippet actions()}<DiscoverActions kind="movies" {tmdbId} ondownload={(r) => (sources = r)} />{/snippet}
			</TitleHero>
		{/snippet}
		<MoreLikeThis kind="movies" {tmdbId} />
	</DetailLayout>
{/key}

<SourcesDialog title={sources?.title} onclose={() => (sources = null)}>
	{#snippet children(heading)}
		{#if sources}<ReleaseTable target={sources.target} {heading} auto />{/if}
	{/snippet}
</SourcesDialog>
