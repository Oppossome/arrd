<script lang="ts">
	import { page } from '$app/state';
	import { QueueTable, ReleaseTable, SeasonBrowser, SeriesActions, TitleHero } from '#lib/components/organisms/index.js';
	import { DetailLayout, Section, SourcesDialog } from '#lib/components/templates/index.js';
	import type { SourcesRequest } from '#lib/domain/releases.js';

	const id = $derived(Number(page.params.id));
	let sources = $state<SourcesRequest | null>(null);
</script>

{#key id}
	<DetailLayout bleed>
		{#snippet header()}
			<TitleHero kind="series" libraryId={id}>
				{#snippet actions()}<SeriesActions {id} ondownload={(r) => (sources = r)} />{/snippet}
			</TitleHero>
		{/snippet}
		<SeasonBrowser seriesId={id} ondownload={(r) => (sources = r)} />
		<Section title="Downloads">
			<QueueTable filter={{ seriesId: id }} compact />
		</Section>
	</DetailLayout>
{/key}

<SourcesDialog title={sources?.title} onclose={() => (sources = null)}>
	{#snippet children(heading)}
		{#if sources}<ReleaseTable target={sources.target} {heading} auto />{/if}
	{/snippet}
</SourcesDialog>
