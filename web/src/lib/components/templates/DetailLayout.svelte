<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		header: Snippet;
		actions?: Snippet;
		/** Run the header to the edges of the content area (for a backdrop hero). */
		bleed?: boolean;
		children: Snippet;
	}
	let { header, actions, bleed = false, children }: Props = $props();
</script>

<div class="detail">
	<header class="top" class:bleed>
		<div class="header">{@render header()}</div>
		{#if actions}<div class="actions">{@render actions()}</div>{/if}
	</header>
	<div class="body">{@render children()}</div>
</div>

<style>
	.detail {
		display: grid;
		gap: var(--space-6);
	}
	.top {
		display: grid;
		gap: var(--space-4);
	}
	/* Cancels AppShell's content padding (space-6 / space-5). */
	.bleed {
		margin: calc(-1 * var(--space-6)) calc(-1 * var(--space-5)) 0;
	}
	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-2);
	}
	.body {
		display: grid;
		gap: var(--space-6);
		min-width: 0;
	}
</style>
