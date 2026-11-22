<script lang="ts" generics="K extends string">
	interface Props {
		tabs: { key: K; label: string }[];
		active: K;
		label: string;
		onselect: (key: K) => void;
	}
	let { tabs, active, label, onselect }: Props = $props();
</script>

<div class="tabs" role="tablist" aria-label={label}>
	{#each tabs as tab (tab.key)}
		<button
			type="button"
			role="tab"
			class:active={tab.key === active}
			aria-selected={tab.key === active}
			onclick={() => onselect(tab.key)}>{tab.label}</button
		>
	{/each}
</div>

<style>
	.tabs {
		display: flex;
		gap: var(--space-1);
		border-bottom: 1px solid var(--color-border);
		overflow-x: auto;
	}
	button {
		padding: var(--space-2) var(--space-3);
		border: 0;
		border-bottom: 2px solid transparent;
		background: transparent;
		color: var(--color-text-muted);
		font: inherit;
		font-size: var(--text-sm);
		cursor: pointer;
		white-space: nowrap;
	}
	.active {
		color: var(--color-text);
		border-bottom-color: var(--color-accent);
	}
</style>
