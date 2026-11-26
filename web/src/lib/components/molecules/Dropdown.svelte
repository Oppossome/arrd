<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Icon } from '#lib/components/atoms/index.js';

	interface Props {
		/** Names the panel for assistive technology. */
		label: string;
		/** Draws the trigger with the accent border, e.g. while a filter is on. */
		active?: boolean;
		/** The trigger's content; a chevron follows it. */
		button: Snippet;
		/** The panel's content, given a function that closes the menu. */
		children: Snippet<[close: () => void]>;
	}
	let { label, active = false, button, children }: Props = $props();

	const panelId = $props.id();
	// Without the Popover API (older browsers, jsdom) the panel is still fixed
	// in place, which escapes a scrolling container; only the top layer is lost.
	const popover = typeof HTMLElement.prototype.showPopover === 'function';
	let open = $state(false);
	let root = $state<HTMLDivElement>();
	let trigger = $state<HTMLButtonElement>();
	let panel = $state<HTMLDivElement>();

	function close() {
		open = false;
		trigger?.focus();
	}

	// The panel opens in the top layer, placed under the trigger, so a
	// scrolling or clipped container around the menu cannot cut it off.
	$effect(() => {
		if (!panel || !trigger) return;
		if (popover) panel.showPopover();
		const gap = 4;
		const rect = trigger.getBoundingClientRect();
		const left = Math.max(gap, Math.min(rect.left, window.innerWidth - panel.offsetWidth - gap));
		panel.style.left = `${left}px`;
		panel.style.top = `${rect.bottom + gap}px`;
	});

	// While open, a press outside the menu, a scroll outside the panel or
	// a resize closes it: the panel would no longer sit by its trigger.
	$effect(() => {
		if (!open) return;
		const dismiss = () => (open = false);
		const onpointerdown = (e: PointerEvent) => {
			if (root && !root.contains(e.target as Node)) dismiss();
		};
		const onscroll = (e: Event) => {
			if (!panel?.contains(e.target as Node)) dismiss();
		};
		document.addEventListener('pointerdown', onpointerdown);
		document.addEventListener('scroll', onscroll, true);
		window.addEventListener('resize', dismiss);
		return () => {
			document.removeEventListener('pointerdown', onpointerdown);
			document.removeEventListener('scroll', onscroll, true);
			window.removeEventListener('resize', dismiss);
		};
	});

	function onkeydown(e: KeyboardEvent) {
		if (e.key !== 'Escape' || !open) return;
		// Close only the menu, not a dialog around it.
		e.preventDefault();
		e.stopPropagation();
		close();
	}
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="menu" bind:this={root} {onkeydown}>
	<button
		type="button"
		class="trigger"
		class:active
		aria-haspopup="true"
		aria-expanded={open}
		aria-controls={open ? panelId : undefined}
		bind:this={trigger}
		onclick={() => (open = !open)}
	>
		{@render button()}
		<span class="chevron" class:open><Icon name="chevron" size={14} /></span>
	</button>

	{#if open}
		<div
			class="panel"
			id={panelId}
			role="group"
			aria-label={label}
			popover={popover ? 'manual' : undefined}
			bind:this={panel}
		>
			{@render children(close)}
		</div>
	{/if}
</div>

<style>
	.menu {
		flex-shrink: 0;
	}
	.trigger {
		display: inline-flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-1) var(--space-2) var(--space-1) var(--space-3);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-pill);
		background: var(--color-surface);
		color: var(--color-text-muted);
		font: inherit;
		font-size: var(--text-xs);
		white-space: nowrap;
		cursor: pointer;
		transition:
			background var(--transition-fast),
			color var(--transition-fast),
			border-color var(--transition-fast);
	}
	.trigger:hover,
	.trigger[aria-expanded='true'] {
		background: var(--color-surface-raised);
		color: var(--color-text);
	}
	.active {
		border-color: var(--color-accent);
		background: var(--color-surface-raised);
		color: var(--color-text);
	}
	.chevron {
		display: inline-flex;
		transform: rotate(90deg);
		transition: transform var(--transition-fast);
	}
	.chevron.open {
		transform: rotate(-90deg);
	}
	/* Placed by script in viewport coordinates (see the effect above). */
	.panel {
		position: fixed;
		inset: auto;
		z-index: 10;
		min-width: 10rem;
		margin: 0;
		padding: 0;
		overflow: hidden;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-md);
		background: var(--color-surface-raised);
		color: var(--color-text);
		font-size: var(--text-sm);
		box-shadow: 0 8px 24px rgb(0 0 0 / 0.3);
	}
</style>
