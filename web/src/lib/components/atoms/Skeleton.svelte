<script lang="ts">
	interface Props {
		height?: string;
		width?: string;
	}
	let { height = '1rem', width = '100%' }: Props = $props();
</script>

<span class="skeleton" style:height style:width aria-hidden="true"></span>

<style>
	.skeleton {
		display: block;
		border-radius: var(--radius-sm);
		background: linear-gradient(90deg, var(--color-surface) 0%, var(--color-surface-raised) 50%, var(--color-surface) 100%);
		background-size: 200% 100%;
		animation: shimmer 1.4s ease-in-out infinite;
	}
	@keyframes shimmer {
		from {
			background-position: 100% 0;
		}
		to {
			background-position: -100% 0;
		}
	}
</style>
