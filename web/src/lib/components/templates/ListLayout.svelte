<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Heading, Icon } from '#lib/components/atoms/index.js';

	interface Props {
		title: string;
		/** A primary link in the header, e.g. "Add movie". */
		action?: { href: string; label: string };
		toolbar?: Snippet;
		children: Snippet;
	}
	let { title, action, toolbar, children }: Props = $props();
</script>

<div class="list-layout">
	<header class="header">
		<Heading level={1}>{title}</Heading>
		<div class="toolbar">
			{@render toolbar?.()}
			{#if action}<a class="action" href={action.href}><Icon name="plus" />{action.label}</a>{/if}
		</div>
	</header>
	{@render children()}
</div>

<style>
	.list-layout {
		display: grid;
		gap: var(--space-5);
	}
	.header {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-3);
	}
	.toolbar {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-2);
	}
	.action {
		display: inline-flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-4);
		border-radius: var(--radius-md);
		background: var(--color-accent-strong);
		color: var(--color-on-accent);
		font-size: var(--text-sm);
		font-weight: var(--weight-medium);
		text-decoration: none;
	}
	.action:hover {
		background: var(--color-accent);
	}
</style>
