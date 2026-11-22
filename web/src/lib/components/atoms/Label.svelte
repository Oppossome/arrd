<script lang="ts">
	import type { Snippet } from 'svelte';
	interface Props {
		for?: string;
		children: Snippet;
	}
	let { for: htmlFor, children }: Props = $props();
</script>

<label for={htmlFor}>{@render children()}</label>

<style>
	label {
		font-size: var(--text-sm);
		font-weight: var(--weight-medium);
		color: var(--color-text);
	}
</style>
