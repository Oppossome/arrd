<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Heading, IconButton, Rating, Text } from '#lib/components/atoms/index.js';
	import { formatRuntime } from '#lib/format.js';

	interface Props {
		title: string;
		year?: number;
		overview: string;
		tagline?: string;
		backdropSrc?: string;
		/** YouTube id of a trailer, played muted behind the title. */
		trailerKey?: string;
		/** Score out of 10. */
		rating?: number;
		votes?: number;
		certification?: string;
		/** Minutes. */
		runtime?: number;
		genres?: string[];
		networks?: string[];
		/** Status badges beside the title. */
		badges?: Snippet;
		/** Buttons and controls under the overview. */
		actions?: Snippet;
	}
	let {
		title,
		year,
		overview,
		tagline,
		backdropSrc,
		trailerKey,
		rating,
		votes,
		certification,
		runtime,
		genres = [],
		networks = [],
		badges,
		actions
	}: Props = $props();

	const TRAILER_ORIGIN = 'https://www.youtube-nocookie.com';
	/** Let the backdrop settle before the trailer fades in over it. */
	const TRAILER_DELAY_MS = 1500;

	let playing = $state(false);
	let loaded = $state(false);
	let muted = $state(true);
	let frame = $state<HTMLIFrameElement>();

	// Autoplay unless the viewer prefers reduced motion; they can still start it.
	$effect(() => {
		if (!trailerKey) return;
		if (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return;
		const t = setTimeout(() => (playing = true), TRAILER_DELAY_MS);
		return () => clearTimeout(t);
	});

	const trailerSrc = $derived.by(() => {
		if (!trailerKey) return undefined;
		const params = new URLSearchParams({
			autoplay: '1',
			mute: '1',
			controls: '0',
			loop: '1', // looping a single video needs it listed as the playlist
			playlist: trailerKey,
			playsinline: '1',
			rel: '0',
			disablekb: '1',
			enablejsapi: '1', // for the mute/unmute commands
			origin: window.location.origin
		});
		return `${TRAILER_ORIGIN}/embed/${encodeURIComponent(trailerKey)}?${params.toString()}`;
	});

	/** Sends a command to the YouTube player (needs enablejsapi=1). */
	function command(func: 'mute' | 'unMute') {
		frame?.contentWindow?.postMessage(JSON.stringify({ event: 'command', func, args: [] }), TRAILER_ORIGIN);
	}
	function toggleMute() {
		muted = !muted;
		command(muted ? 'mute' : 'unMute');
	}
	function togglePlaying() {
		playing = !playing;
		loaded = false;
		muted = true;
	}

	const meta = $derived([year ? String(year) : '', formatRuntime(runtime), ...networks].filter(Boolean));
</script>

<section class="hero" aria-label={title}>
	<div class="art" aria-hidden="true">
		{#if backdropSrc}<img class="backdrop" src={backdropSrc} alt="" />{/if}
		{#if playing && trailerSrc}
			<iframe
				bind:this={frame}
				class="trailer"
				class:loaded
				src={trailerSrc}
				title="{title} trailer"
				tabindex="-1"
				allow="autoplay; encrypted-media; picture-in-picture"
				referrerpolicy="strict-origin-when-cross-origin"
				onload={() => (loaded = true)}
			></iframe>
		{/if}
		<div class="shade"></div>
	</div>

	<div class="content">
		<div class="title">
			<Heading level={1}>{title}</Heading>
			{@render badges?.()}
		</div>
		<div class="meta">
			{#if rating}<Rating value={rating} {votes} />{/if}
			{#if certification}<span class="cert">{certification}</span>{/if}
			{#each meta as m (m)}<Text as="span" size="sm" tone="muted">{m}</Text>{/each}
		</div>
		{#if genres.length > 0}
			<Text size="sm">{genres.join(' · ')}</Text>
		{/if}
		{#if tagline}<p class="tagline">{tagline}</p>{/if}
		<div class="overview"><Text size="sm">{overview}</Text></div>
		{#if actions}<div class="actions">{@render actions()}</div>{/if}
	</div>

	{#if trailerKey}
		<div class="controls">
			{#if playing}
				<IconButton
					icon={muted ? 'volume-off' : 'volume'}
					label={muted ? 'Unmute trailer' : 'Mute trailer'}
					onclick={toggleMute}
				/>
			{/if}
			<IconButton icon={playing ? 'pause' : 'play'} label={playing ? 'Stop trailer' : 'Play trailer'} onclick={togglePlaying} />
		</div>
	{/if}
</section>

<style>
	.hero {
		position: relative;
		display: grid;
		align-items: end;
		min-height: clamp(22rem, 52vh, 36rem);
		overflow: hidden;
		background: var(--color-surface);
		isolation: isolate;
	}
	.art {
		position: absolute;
		inset: 0;
		z-index: -1;
		overflow: hidden;
	}
	.backdrop {
		width: 100%;
		height: 100%;
		object-fit: cover;
		object-position: center 20%;
		display: block;
	}
	/* Cover the hero with a 16:9 player, cropping its edges like the backdrop. */
	.trailer {
		position: absolute;
		top: 50%;
		left: 50%;
		width: max(100%, calc(100% * 16 / 9));
		min-width: 100%;
		min-height: 100%;
		aspect-ratio: 16 / 9;
		transform: translate(-50%, -50%) scale(1.15);
		border: 0;
		pointer-events: none;
		opacity: 0;
		transition: opacity 800ms ease;
	}
	.trailer.loaded {
		opacity: 1;
	}
	.shade {
		position: absolute;
		inset: 0;
		background:
			linear-gradient(to right, var(--color-bg) 0%, transparent 65%),
			linear-gradient(to top, var(--color-bg) 0%, transparent 55%);
	}
	.content {
		display: grid;
		gap: var(--space-3);
		max-width: 42rem;
		padding: var(--space-7) var(--space-5) var(--space-6);
	}
	.title {
		display: flex;
		flex-wrap: wrap;
		align-items: baseline;
		gap: var(--space-3);
	}
	.meta {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-3);
	}
	.cert {
		padding: 0 var(--space-1);
		border: 1px solid var(--color-text-muted);
		border-radius: var(--radius-sm);
		color: var(--color-text-muted);
		font-size: var(--text-xs);
		line-height: 1.5;
	}
	.tagline {
		margin: 0;
		color: var(--color-text-muted);
		font-size: var(--text-sm);
		font-style: italic;
	}
	.overview {
		display: -webkit-box;
		-webkit-line-clamp: 4;
		line-clamp: 4;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}
	.actions {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-3);
		padding-top: var(--space-2);
	}
	.controls {
		position: absolute;
		right: var(--space-5);
		bottom: var(--space-6);
		display: flex;
		gap: var(--space-2);
	}
	.controls :global(button) {
		border: 1px solid var(--color-text-muted);
		border-radius: var(--radius-pill);
		background: var(--color-overlay);
		color: var(--color-text);
	}
	@media (max-width: 40rem) {
		.content {
			padding-top: var(--space-7);
		}
		.controls {
			top: var(--space-4);
			bottom: auto;
		}
	}
</style>
