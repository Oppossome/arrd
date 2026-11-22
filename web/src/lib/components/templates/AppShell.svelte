<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		sidebar: Snippet;
		banner?: Snippet;
		children: Snippet;
	}
	let { sidebar, banner, children }: Props = $props();
</script>

<div class="shell">
	<aside class="sidebar">{@render sidebar()}</aside>
	<div class="main">
		{@render banner?.()}
		<main class="content">{@render children()}</main>
	</div>
</div>

<style>
	.shell {
		display: grid;
		grid-template-columns: var(--sidebar-width) 1fr;
		min-height: 100vh;
	}
	.sidebar {
		border-right: 1px solid var(--color-border);
		background: var(--color-surface);
	}
	.main {
		display: grid;
		grid-template-rows: auto 1fr;
		min-width: 0;
	}
	.content {
		width: 100%;
		max-width: var(--content-max);
		margin: 0 auto;
		padding: var(--space-6) var(--space-5);
	}
	@media (max-width: 48rem) {
		.shell {
			grid-template-columns: 1fr;
		}
		.sidebar {
			border-right: 0;
			border-bottom: 1px solid var(--color-border);
		}
	}
</style>
