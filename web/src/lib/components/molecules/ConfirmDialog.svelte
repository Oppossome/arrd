<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Button, Heading } from '#lib/components/atoms/index.js';

	interface Props {
		open: boolean;
		title: string;
		confirmLabel?: string;
		danger?: boolean;
		busy?: boolean;
		children?: Snippet;
		onconfirm: () => void;
		oncancel: () => void;
	}
	let { open, title, confirmLabel = 'Confirm', danger = false, busy = false, children, onconfirm, oncancel }: Props =
		$props();

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
</script>

<dialog bind:this={dialog} aria-label={title} oncancel={(e) => { e.preventDefault(); oncancel(); }}>
	{#if open}
		<div class="body">
			<Heading level={3}>{title}</Heading>
			{@render children?.()}
			<div class="actions">
				<Button onclick={oncancel} disabled={busy}>Cancel</Button>
				<Button variant={danger ? 'danger' : 'primary'} loading={busy} onclick={onconfirm}>{confirmLabel}</Button>
			</div>
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
		max-width: min(32rem, 90vw);
	}
	dialog::backdrop {
		background: var(--color-overlay);
	}
	.body {
		display: grid;
		gap: var(--space-4);
		padding: var(--space-5);
	}
	.actions {
		display: flex;
		justify-content: flex-end;
		gap: var(--space-2);
	}
</style>
