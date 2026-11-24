<script lang="ts">
	import { Text } from '#lib/components/atoms/index.js';

	interface CastCard {
		name: string;
		character?: string;
		photoSrc?: string;
	}
	interface Props {
		cast: CastCard[];
	}
	let { cast }: Props = $props();
	let failed = $state<Record<number, boolean>>({});
</script>

{#if cast.length > 0}
	<ul class="cast" aria-label="Cast">
		{#each cast as c, i (i)}
			<li>
				<div class="photo">
					{#if c.photoSrc && !failed[i]}
						<img src={c.photoSrc} alt="" loading="lazy" onerror={() => (failed[i] = true)} />
					{:else}
						<span aria-hidden="true">{c.name.slice(0, 1).toUpperCase()}</span>
					{/if}
				</div>
				<Text size="sm" as="span">{c.name}</Text>
				{#if c.character}<Text size="xs" tone="muted" as="span">{c.character}</Text>{/if}
			</li>
		{/each}
	</ul>
{/if}

<style>
	.cast {
		display: grid;
		grid-auto-flow: column;
		grid-auto-columns: 6.5rem;
		gap: var(--space-4);
		margin: 0;
		padding: 0 0 var(--space-2);
		list-style: none;
		overflow-x: auto;
		overscroll-behavior-x: contain;
		scroll-snap-type: x mandatory;
		scrollbar-width: thin;
	}
	li {
		display: grid;
		justify-items: center;
		align-content: start;
		gap: var(--space-1);
		text-align: center;
		scroll-snap-align: start;
	}
	.photo {
		display: grid;
		place-items: center;
		width: 5rem;
		aspect-ratio: 1;
		border-radius: var(--radius-pill);
		overflow: hidden;
		background: var(--color-surface-raised);
		color: var(--color-text-muted);
		font-size: var(--text-xl);
		font-weight: var(--weight-bold);
	}
	img {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}
</style>
