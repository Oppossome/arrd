<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Heading, IconButton } from '#lib/components/atoms/index.js';

	interface Props {
		open: boolean;
		title: string;
		/** Called on Escape, the close button or a click outside the dialog. */
		onclose: () => void;
		/**
		 * The content draws its own header and padding; only the close button
		 * is added, at the top right. The title still names the dialog.
		 */
		bare?: boolean;
		children: Snippet;
	}
	let { open, title, onclose, bare = false, children }: Props = $props();

	let dialog = $state<HTMLDialogElement>();
	$effect(() => {
		if (!dialog) return;
		if (open && !dialog.open) {
			// Fall back to a plain open dialog where showModal is unavailable.
			if (typeof dialog.showModal === 'function') dialog.showModal();
			else dialog.setAttribute('open', '');
		}
		if (!open && dialog.open) {
			if (typeof dialog.close === 'function') dialog.close();
			else dialog.removeAttribute('open');
		}
	});

	// A click on the dialog element itself (not its contents) is on the backdrop.
	function onclick(e: MouseEvent) {
		if (e.target === dialog) onclose();
	}
</script>

<dialog
	bind:this={dialog}
	aria-label={title}
	{onclick}
	oncancel={(e) => {
		e.preventDefault();
		onclose();
	}}
>
	{#if open}
		<div class="frame" class:bare>
			{#if bare}
				<div class="close"><IconButton icon="x" label="Close" onclick={onclose} /></div>
			{:else}
				<div class="head">
					<Heading level={3}>{title}</Heading>
					<IconButton icon="x" label="Close" onclick={onclose} />
				</div>
			{/if}
			<div class="body">{@render children()}</div>
		</div>
	{/if}
</dialog>

<style>
	dialog {
		padding: 0;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-lg);
		background: var(--color-surface);
		color: var(--color-text);
		box-shadow: var(--shadow-raised);
		width: min(64rem, 94vw);
		max-height: 88vh;
	}
	dialog::backdrop {
		background: var(--color-overlay);
	}
	.frame {
		display: grid;
		grid-template-rows: auto 1fr;
		max-height: 88vh;
	}
	.head {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-4) var(--space-5);
		border-bottom: 1px solid var(--color-border);
	}
	.body {
		overflow: auto;
		padding: var(--space-5);
	}
	.bare {
		position: relative;
		grid-template-rows: 1fr;
	}
	.bare .body {
		padding: 0;
	}
	/* Sits over the content's header row, which leaves room for it. */
	.close {
		position: absolute;
		top: var(--space-3);
		right: var(--space-4);
		z-index: 2;
	}

	/* On phones the dialog becomes a bottom drawer: full width, pinned to the
	   bottom edge, sliding up when opened. */
	@media (max-width: 40rem) {
		dialog {
			width: 100%;
			max-width: 100%;
			margin: auto 0 0;
			border-bottom: none;
			border-radius: var(--radius-lg) var(--radius-lg) 0 0;
			max-height: 92dvh;
		}
		dialog[open] {
			animation: drawer-in 240ms cubic-bezier(0.32, 0.72, 0, 1);
		}
		dialog[open]::backdrop {
			animation: backdrop-in 240ms ease-out;
		}
		.frame {
			max-height: 92dvh;
		}
		.body {
			padding-bottom: calc(var(--space-5) + env(safe-area-inset-bottom, 0px));
		}
		.bare .body {
			padding-bottom: env(safe-area-inset-bottom, 0px);
		}
	}
	@media (max-width: 40rem) and (prefers-reduced-motion: reduce) {
		dialog[open] {
			animation: none;
		}
	}
	@keyframes drawer-in {
		from {
			transform: translateY(100%);
		}
	}
	@keyframes backdrop-in {
		from {
			opacity: 0;
		}
	}
</style>
