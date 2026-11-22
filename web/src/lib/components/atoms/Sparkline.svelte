<script lang="ts">
	interface Props {
		values: number[];
		label: string;
		/** Top of the scale; defaults to the largest value. */
		max?: number;
		tone?: 'info' | 'success';
	}
	let { values, label, max, tone = 'info' }: Props = $props();

	const W = 100;
	const H = 24;
	const top = $derived(Math.max(max ?? 0, ...values, 1));
	const points = $derived(
		values.map((v, i) => `${values.length > 1 ? (i / (values.length - 1)) * W : W},${H - (Math.max(0, v) / top) * H}`)
	);
</script>

<svg class="spark {tone}" viewBox="0 0 {W} {H}" preserveAspectRatio="none" role="img" aria-label={label}>
	{#if points.length > 0}
		<polygon class="area" points="0,{H} {points.join(' ')} {W},{H}" />
		<polyline class="line" points={points.join(' ')} />
	{/if}
</svg>

<style>
	.spark {
		display: block;
		width: 100%;
		height: 2rem;
	}
	.line {
		fill: none;
		stroke: currentColor;
		stroke-width: 1.5;
		vector-effect: non-scaling-stroke;
	}
	.area {
		fill: currentColor;
		opacity: 0.15;
	}
	.info {
		color: var(--color-info);
	}
	.success {
		color: var(--color-success);
	}
</style>
