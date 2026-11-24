<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Modal } from '#lib/components/molecules/index.js';

	interface Props {
		/** What the sources are for; the dialog is closed when absent. */
		title?: string;
		onclose: () => void;
		/**
		 * The release search (an organism the page supplies), given the heading
		 * to show: it draws the dialog's header with its own controls.
		 */
		children: Snippet<[string]>;
	}
	let { title, onclose, children }: Props = $props();

	const heading = $derived(title ? `Sources · ${title}` : 'Sources');
</script>

<Modal open={title !== undefined} title={heading} {onclose} bare>
	{#if title !== undefined}{@render children(heading)}{/if}
</Modal>
