<script lang="ts">
	import { page } from '$app/state';
	import { DiscoverActions, MoreLikeThis, ReleaseTable, SeasonBrowser, TitleHero } from '#lib/components/organisms/index.js';
	import { DetailLayout, SourcesDialog } from '#lib/components/templates/index.js';
	import type { SourcesRequest } from '#lib/releases.js';

	const tmdbId = $derived(Number(page.params.tmdbId));
	let sources = $state<SourcesRequest | null>(null);
</script>

{#key tmdbId}
	<DetailLayout bleed>
		{#snippet header()}
			<TitleHero kind="series" {tmdbId}>
				{#snippet actions()}<DiscoverActions kind="series" {tmdbId} />{/snippet}
			</TitleHero>
		{/snippet}
		<SeasonBrowser {tmdbId} ondownload={(r) => (sources = r)} />
		<MoreLikeThis kind="series" {tmdbId} />
	</DetailLayout>
{/key}

<SourcesDialog title={sources?.title} onclose={() => (sources = null)}>
	{#snippet children(heading)}
		{#if sources}<ReleaseTable source={sources.source} {heading} auto />{/if}
	{/snippet}
</SourcesDialog>
