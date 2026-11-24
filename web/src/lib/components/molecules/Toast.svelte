<script lang="ts">
	import { IconButton, Text } from '#lib/components/atoms/index.js';
	import type { ToastKind } from '#lib/stores/toast.svelte.js';

	interface Props {
		kind: ToastKind;
		text: string;
		ondismiss: () => void;
	}
	let { kind, text, ondismiss }: Props = $props();
</script>

<div class="toast {kind}" role={kind === 'error' ? 'alert' : 'status'}>
	<Text size="sm" as="span">{text}</Text>
	<IconButton icon="x" label="Dismiss" onclick={ondismiss} />
</div>

<style>
	.toast {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-3);
		padding: var(--space-2) var(--space-2) var(--space-2) var(--space-4);
		border: 1px solid var(--color-border);
		border-left: 4px solid var(--color-info);
		border-radius: var(--radius-md);
		background: var(--color-surface-raised);
		box-shadow: var(--shadow-raised);
	}
	.success {
		border-left-color: var(--color-success);
	}
	.error {
		border-left-color: var(--color-danger);
	}
</style>
