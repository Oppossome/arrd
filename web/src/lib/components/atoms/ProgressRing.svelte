<script lang="ts">
	interface Props {
		/** 0–100 */
		value: number;
		label: string;
		tone?: 'info' | 'success' | 'neutral';
		/** Diameter in rem. */
		size?: number;
	}
	let { value, label, tone = 'info', size = 2.5 }: Props = $props();
	const clamped = $derived(Math.max(0, Math.min(100, value)));
	const R = 16;
	const CIRCUMFERENCE = 2 * Math.PI * R;
</script>

<div
	class="ring {tone}"
	style:width="{size}rem"
	style:height="{size}rem"
	role="progressbar"
	aria-label={label}
	aria-valuemin={0}
	aria-valuemax={100}
	aria-valuenow={clamped}
>
	<svg viewBox="0 0 40 40" aria-hidden="true">
		<circle class="track" cx="20" cy="20" r={R} />
		<circle
			class="fill"
			cx="20"
			cy="20"
			r={R}
			stroke-dasharray={CIRCUMFERENCE}
			stroke-dashoffset={CIRCUMFERENCE * (1 - clamped / 100)}
		/>
	</svg>
	<span class="value">{Math.floor(clamped)}%</span>
</div>

<style>
	.ring {
		position: relative;
		display: grid;
		place-items: center;
		border-radius: var(--radius-pill);
		background: var(--color-overlay);
	}
	svg {
		position: absolute;
		inset: 0;
		transform: rotate(-90deg);
	}
	circle {
		fill: none;
		stroke-width: 4;
	}
	.track {
		stroke: var(--color-surface-raised);
	}
	.fill {
		stroke-linecap: round;
		transition: stroke-dashoffset var(--transition-fast);
	}
	.info .fill {
		stroke: var(--color-info);
	}
	.success .fill {
		stroke: var(--color-success);
	}
	.neutral .fill {
		stroke: var(--color-neutral);
	}
	.value {
		color: var(--color-text);
		font-size: var(--text-xs);
		font-weight: var(--weight-bold);
	}
</style>
