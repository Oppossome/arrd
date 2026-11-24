<script lang="ts">
	import { page } from '$app/state';
	import { MovieActions, QueueTable, ReleaseTable, TitleHero } from '#lib/components/organisms/index.js';
	import { DetailLayout, Section, SourcesDialog } from '#lib/components/templates/index.js';
	import type { SourcesRequest } from '#lib/releases.js';

	const id = $derived(Number(page.params.id));
	let sources = $state<SourcesRequest | null>(null);
</script>

{#key id}
	<DetailLayout bleed>
		{#snippet header()}
			<TitleHero kind="movies" libraryId={id}>
				{#snippet actions()}<MovieActions {id} ondownload={(r) => (sources = r)} />{/snippet}
			</TitleHero>
		{/snippet}
		<Section title="Downloads">
			<QueueTable filter={{ movieId: id }} compact />
		</Section>
	</DetailLayout>
{/key}

<SourcesDialog title={sources?.title} onclose={() => (sources = null)}>
	{#snippet children(heading)}
		{#if sources}<ReleaseTable source={sources.source} {heading} auto />{/if}
	{/snippet}
</SourcesDialog>
