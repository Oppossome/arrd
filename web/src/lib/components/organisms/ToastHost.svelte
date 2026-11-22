<script lang="ts">
	import { Toast } from '#lib/components/molecules/index.js';
	import { toasts } from '#lib/toast.svelte.js';
</script>

<div class="toasts" aria-live="polite">
	{#each toasts.items as t (t.id)}
		<Toast kind={t.kind} text={t.text} ondismiss={() => toasts.dismiss(t.id)} />
	{/each}
</div>

<style>
	.toasts {
		position: fixed;
		right: var(--space-4);
		bottom: var(--space-4);
		z-index: 10;
		display: grid;
		gap: var(--space-2);
		width: min(24rem, calc(100vw - 2 * var(--space-4)));
	}
</style>
