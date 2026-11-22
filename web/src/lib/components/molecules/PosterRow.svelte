<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Heading, IconButton } from '#lib/components/atoms/index.js';

	interface Props {
		title: string;
		/** The row's cards, laid out side by side. */
		children: Snippet;
	}
	let { title, children }: Props = $props();

	let track = $state<HTMLDivElement>();
	let atStart = $state(true);
	let atEnd = $state(true);

	function measure() {
		if (!track) return;
		atStart = track.scrollLeft <= 1;
		atEnd = track.scrollLeft + track.clientWidth >= track.scrollWidth - 1;
	}
	$effect(measure);

	/** Scrolls by most of a screenful, so the last card stays in view. */
	function page(direction: 1 | -1) {
		track?.scrollBy({ left: direction * track.clientWidth * 0.9, behavior: 'smooth' });
	}
</script>

<svelte:window onresize={measure} />

<section class="row" aria-label={title}>
	<div class="head">
		<Heading level={2}>{title}</Heading>
		<div class="nav">
			<IconButton icon="chevron-left" label="Scroll back" disabled={atStart} onclick={() => page(-1)} />
			<IconButton icon="chevron" label="Scroll forward" disabled={atEnd} onclick={() => page(1)} />
		</div>
	</div>
	<div class="track" bind:this={track} onscroll={measure}>
		{@render children()}
	</div>
</section>

<style>
	.row {
		display: grid;
		gap: var(--space-3);
		min-width: 0;
	}
	.head {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: var(--space-3);
	}
	.nav {
		display: flex;
		gap: var(--space-1);
	}
	.track {
		display: grid;
		grid-auto-flow: column;
		grid-auto-columns: 9.5rem;
		gap: var(--space-4);
		overflow-x: auto;
		overscroll-behavior-x: contain;
		scroll-snap-type: x mandatory;
		scrollbar-width: thin;
		padding-bottom: var(--space-2);
	}
	.track > :global(*) {
		scroll-snap-align: start;
	}
</style>
