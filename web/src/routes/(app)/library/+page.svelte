<script lang="ts">
	import { ActiveDownloads, PosterGrid, RSSSyncButton } from '#lib/components/organisms/index.js';
	import { ListLayout, Section } from '#lib/components/templates/index.js';
</script>

<ListLayout title="Library">
	{#snippet toolbar()}<RSSSyncButton />{/snippet}
	<ActiveDownloads />
	<Section title="Movies">
		<PosterGrid kind="movies" />
	</Section>
	<Section title="Shows">
		<PosterGrid kind="series" />
	</Section>
</ListLayout>
