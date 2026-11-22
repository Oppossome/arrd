<script lang="ts">
	import { Icon, Text } from '#lib/components/atoms/index.js';

	let { error }: { error: unknown } = $props();
	const message = $derived(error instanceof Error ? error.message : String(error));
</script>

<div class="error" role="alert">
	<Icon name="alert" />
	<Text tone="danger" size="sm" as="span">{message}</Text>
</div>

<style>
	.error {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-3);
		border: 1px solid var(--color-danger);
		border-radius: var(--radius-md);
		color: var(--color-danger);
	}
</style>
