<script lang="ts">
	interface Props {
		src?: string;
		alt: string;
	}
	let { src, alt }: Props = $props();
	let failed = $state(false);
</script>

<div class="poster">
	{#if src && !failed}
		<img {src} {alt} loading="lazy" onerror={() => (failed = true)} />
	{:else}
		<span class="placeholder" role="img" aria-label={alt}>{alt.slice(0, 1).toUpperCase()}</span>
	{/if}
</div>

<style>
	.poster {
		aspect-ratio: var(--poster-ratio);
		border-radius: var(--radius-md);
		overflow: hidden;
		background: var(--color-surface-raised);
	}
	img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		display: block;
	}
	.placeholder {
		display: grid;
		place-items: center;
		height: 100%;
		color: var(--color-text-muted);
		font-size: var(--text-2xl);
		font-weight: var(--weight-bold);
	}
</style>
