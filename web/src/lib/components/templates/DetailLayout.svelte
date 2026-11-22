<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		header: Snippet;
		actions?: Snippet;
		children: Snippet;
	}
	let { header, actions, children }: Props = $props();
</script>

<div class="detail">
	<header class="top">
		<div class="header">{@render header()}</div>
		<div class="actions">{@render actions?.()}</div>
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
	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-2);
	}
	.body {
		display: grid;
		gap: var(--space-6);
	}
</style>
