<script lang="ts">
	import type { Snippet } from 'svelte';
	interface Props {
		href: string;
		children: Snippet;
	}
	let { href, children }: Props = $props();
</script>

<a {href}>{@render children()}</a>

<style>
	a {
		color: var(--color-accent);
		text-decoration: none;
	}
	a:hover {
		text-decoration: underline;
	}
</style>
