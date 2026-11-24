<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { Checkbox, Heading, Text } from '#lib/components/atoms/index.js';
	import { ConfirmDialog, DownloadCard, ErrorMessage } from '#lib/components/molecules/index.js';
	import PeerPopover from './PeerPopover.svelte';
	import { activeDownloads } from '#lib/domain/downloads.js';
	import { libraryHref } from '#lib/nav/links.js';
	import { queries, useMutation } from '#lib/api/query.js';
	import type { Download } from '#lib/api/types.js';

	// The same (unfiltered) queue query the poster grids read.
	const queue = createQuery(() => queries.queue());
	const pause = useMutation('pauseDownload');
	const resume = useMutation('resumeDownload');
	const remove = useMutation('removeDownload');

	const active = $derived(activeDownloads(queue.data ?? []));

	let peersOf = $state<Download | null>(null);
	let removing = $state<Download | null>(null);
	let deleteData = $state(false);

	const href = (d: Download) =>
		d.movie_id ? libraryHref('movies', d.movie_id) : d.series_id ? libraryHref('series', d.series_id) : undefined;
	const busy = (d: Download) =>
		(pause.isPending && pause.variables === d.id) || (resume.isPending && resume.variables === d.id);

	function confirmRemove() {
		if (!removing) return;
		remove.mutate({ id: removing.id, deleteData }, { onSuccess: () => (removing = null) });
	}
</script>

{#if queue.isError}
	<ErrorMessage error={queue.error} />
{:else if active.length > 0}
	<section class="shelf" aria-label="Active now">
		<div class="heading">
			<Heading level={2}>Active now</Heading>
			<Text size="sm" tone="muted" as="span">{active.length}</Text>
		</div>
		<div class="cards">
			{#each active as d (d.id)}
				<DownloadCard
					download={d}
					href={href(d)}
					busy={busy(d)}
					onpause={() => pause.mutate(d.id)}
					onresume={() => resume.mutate(d.id)}
					onremove={() => {
						removing = d;
						deleteData = false;
					}}
					onpeers={() => (peersOf = d)}
				/>
			{/each}
		</div>
	</section>
{/if}

<PeerPopover
	downloadId={peersOf?.id ?? null}
	title={peersOf ? peersOf.title || peersOf.release_title : ''}
	onclose={() => (peersOf = null)}
/>

<ConfirmDialog
	open={removing !== null}
	title="Remove download"
	confirmLabel="Remove"
	danger
	busy={remove.isPending}
	onconfirm={confirmRemove}
	oncancel={() => (removing = null)}
>
	<Text size="sm">Stop and forget <strong>{removing?.release_title}</strong>? Files already in your library are kept.</Text>
	<Checkbox bind:checked={deleteData} label="Also delete the downloaded data" />
</ConfirmDialog>

<style>
	.shelf {
		display: grid;
		gap: var(--space-3);
	}
	.heading {
		display: flex;
		align-items: baseline;
		gap: var(--space-2);
	}
	.cards {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(20rem, 1fr));
		gap: var(--space-4);
	}
</style>
