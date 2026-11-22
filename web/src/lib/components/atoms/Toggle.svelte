<script lang="ts">
	interface Props {
		checked: boolean;
		label: string;
		disabled?: boolean;
		onchange?: (checked: boolean) => void;
	}
	let { checked, label, disabled = false, onchange }: Props = $props();
</script>

<button
	type="button"
	role="switch"
	class="toggle"
	class:on={checked}
	aria-checked={checked}
	aria-label={label}
	title={label}
	{disabled}
	onclick={() => onchange?.(!checked)}
>
	<span class="knob"></span>
</button>

<style>
	.toggle {
		position: relative;
		width: 2.25rem;
		height: 1.25rem;
		padding: 0;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-pill);
		background: var(--color-surface-raised);
		cursor: pointer;
		transition: background var(--transition-fast);
	}
	.on {
		background: var(--color-accent-strong);
		border-color: var(--color-accent-strong);
	}
	.knob {
		position: absolute;
		top: 1px;
		left: 1px;
		width: 1rem;
		height: 1rem;
		border-radius: var(--radius-pill);
		background: var(--color-text);
		transition: transform var(--transition-fast);
	}
	.on .knob {
		transform: translateX(1rem);
	}
	.toggle:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
</style>
