<script lang="ts">
	import { page } from '$app/state';
	import { MovieHeader, QueueTable, ReleaseTable } from '#lib/components/organisms/index.js';
	import { DetailLayout, Section } from '#lib/components/templates/index.js';

	const id = $derived(Number(page.params.id));
</script>

{#key id}
	<DetailLayout>
		{#snippet header()}<MovieHeader {id} />{/snippet}
		<Section title="Manual search" description="Search every indexer and pick a release yourself.">
			<ReleaseTable source={{ kind: 'movie', movieId: id }} />
		</Section>
		<Section title="Downloads">
			<QueueTable filter={{ movieId: id }} compact />
		</Section>
	</DetailLayout>
{/key}
