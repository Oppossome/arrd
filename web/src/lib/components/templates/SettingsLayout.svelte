<script lang="ts" generics="K extends string">
	import type { Snippet } from 'svelte';
	import { Heading } from '#lib/components/atoms/index.js';
	import { Tabs } from '#lib/components/molecules/index.js';

	interface Props {
		tabs: { key: K; label: string }[];
		active: K;
		onselect: (key: K) => void;
		children: Snippet;
	}
	let { tabs, active, onselect, children }: Props = $props();
</script>

<div class="settings">
	<Heading level={1}>Settings</Heading>
	<Tabs {tabs} {active} label="Settings sections" {onselect} />
	<section class="panel">{@render children()}</section>
</div>

<style>
	.settings {
		display: grid;
		gap: var(--space-5);
	}
	.panel {
		display: grid;
		gap: var(--space-6);
		max-width: 48rem;
	}
</style>
