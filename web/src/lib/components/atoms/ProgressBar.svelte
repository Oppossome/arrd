<script lang="ts">
	interface Props {
		/** 0–100 */
		value: number;
		label: string;
		tone?: 'info' | 'success' | 'danger';
	}
	let { value, label, tone = 'info' }: Props = $props();
	const clamped = $derived(Math.max(0, Math.min(100, value)));
</script>

<div class="track" role="progressbar" aria-label={label} aria-valuemin={0} aria-valuemax={100} aria-valuenow={clamped}>
	<div class="fill {tone}" style:width="{clamped}%"></div>
</div>

<style>
	.track {
		height: 0.4rem;
		border-radius: var(--radius-pill);
		background: var(--color-surface-raised);
		overflow: hidden;
	}
	.fill {
		height: 100%;
		transition: width var(--transition-fast);
	}
	.info {
		background: var(--color-info);
	}
	.success {
		background: var(--color-success);
	}
	.danger {
		background: var(--color-danger);
	}
</style>
