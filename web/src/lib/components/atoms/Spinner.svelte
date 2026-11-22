<script lang="ts">
	interface Props {
		label?: string;
	}
	let { label = 'Loading' }: Props = $props();
</script>

<span class="spinner" role="status" aria-label={label}></span>

<style>
	.spinner {
		display: inline-block;
		width: 1.25rem;
		height: 1.25rem;
		border: 2px solid var(--color-border);
		border-top-color: var(--color-accent);
		border-radius: var(--radius-pill);
		animation: rot 0.8s linear infinite;
	}
	@keyframes rot {
		to {
			transform: rotate(360deg);
		}
	}
</style>
