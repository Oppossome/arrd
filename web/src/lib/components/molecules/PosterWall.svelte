<script lang="ts">
	import type { Snippet } from 'svelte';

	/** Poster cards (or their skeletons) in a wrapping grid. */
	let { children }: { children: Snippet } = $props();
</script>

<div class="wall">{@render children()}</div>

<style>
	.wall {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(9.5rem, 1fr));
		gap: var(--space-5) var(--space-4);
	}
</style>
