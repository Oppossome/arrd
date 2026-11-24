<script lang="ts">
	import { Toggle } from '#lib/components/atoms/index.js';

	interface Props {
		monitored: boolean;
		what: string;
		disabled?: boolean;
		/** Render as a button-height pill with a fixed "Monitored" caption, for action rows. */
		showLabel?: boolean;
		onchange: (monitored: boolean) => void;
	}
	let { monitored, what, disabled = false, showLabel = false, onchange }: Props = $props();
</script>

{#snippet toggle()}
	<Toggle checked={monitored} label={monitored ? `Stop monitoring ${what}` : `Monitor ${what}`} {disabled} {onchange} />
{/snippet}

{#if showLabel}
	<label class="pill" class:disabled>
		{@render toggle()}
		<span class="caption">Monitored</span>
	</label>
{:else}
	{@render toggle()}
{/if}

<style>
	.pill {
		display: inline-flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-4);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-md);
		background: var(--color-surface-raised);
		cursor: pointer;
		transition: border-color var(--transition-fast);
	}
	.pill:hover:not(.disabled) {
		border-color: var(--color-text-muted);
	}
	.disabled {
		opacity: 0.55;
		cursor: not-allowed;
	}
	.caption {
		color: var(--color-text);
		font-size: var(--text-sm);
		font-weight: var(--weight-medium);
		user-select: none;
	}
</style>
