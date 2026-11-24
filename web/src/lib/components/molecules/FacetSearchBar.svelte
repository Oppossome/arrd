<script lang="ts" module>
	/** A label and value, as a token in the bar or an entry in its suggestion list. */
	export interface FacetItem {
		key: string;
		/** The field, e.g. "Genre". */
		label: string;
		/** Its value, e.g. "Animation". */
		value: string;
	}
</script>

<script lang="ts">
	import { Icon } from '#lib/components/atoms/index.js';

	interface Props {
		/** The filters applied, shown as removable tokens before the input. */
		tokens: FacetItem[];
		/** What the text typed so far could become, best first. */
		suggestions: FacetItem[];
		/** The text being typed. */
		text: string;
		label: string;
		placeholder?: string;
		/** The user picked suggestions[index]. */
		onpick: (index: number) => void;
		/** The user removed a token. */
		onremove: (key: string) => void;
	}
	let { tokens, suggestions, text = $bindable(''), label, placeholder, onpick, onremove }: Props = $props();

	const listId = $props.id();
	let focused = $state(false);
	let dismissed = $state(false);
	let active = $state(0);
	const open = $derived(focused && !dismissed && suggestions.length > 0);

	// New text means a new list: show it from the top.
	$effect(() => {
		void text;
		active = 0;
		dismissed = false;
	});

	function pick(index: number) {
		onpick(index);
		active = 0;
	}

	function onkeydown(e: KeyboardEvent) {
		switch (e.key) {
			case 'ArrowDown':
			case 'ArrowUp':
				if (!open) return;
				e.preventDefault();
				active = (active + (e.key === 'ArrowDown' ? 1 : suggestions.length - 1)) % suggestions.length;
				return;
			case 'Enter':
				if (!open) return;
				e.preventDefault();
				pick(active);
				return;
			case 'Escape':
				dismissed = true;
				return;
			case 'Backspace': {
				const last = tokens.at(-1);
				if (text === '' && last) onremove(last.key);
				return;
			}
		}
	}
</script>

<div class="bar" class:focused>
	<Icon name="search" />
	<ul class="tokens" aria-label="Filters">
		{#each tokens as token (token.key)}
			<li class="token">
				<span class="field">{token.label}</span>
				<span>{token.value}</span>
				<button type="button" aria-label="Remove {token.label} filter" onclick={() => onremove(token.key)}>
					<Icon name="x" size={14} />
				</button>
			</li>
		{/each}
	</ul>
	<input
		type="search"
		role="combobox"
		aria-label={label}
		aria-expanded={open}
		aria-controls={listId}
		aria-autocomplete="list"
		aria-activedescendant={open ? `${listId}-${active}` : undefined}
		{placeholder}
		bind:value={text}
		{onkeydown}
		onfocus={() => (focused = true)}
		onblur={() => (focused = false)}
	/>
	<ul class="suggestions" id={listId} role="listbox" aria-label="Suggestions" hidden={!open}>
		{#each suggestions as s, i (s.key)}
			<!-- Picking keeps focus in the input (mousedown would blur it first).
			     Keyboard users pick from the input, as with any combobox. -->
			<!-- svelte-ignore a11y_click_events_have_key_events -->
			<li
				id="{listId}-{i}"
				role="option"
				aria-selected={i === active}
				class:active={i === active}
				onmousedown={(e) => e.preventDefault()}
				onclick={() => pick(i)}
				onmousemove={() => (active = i)}
			>
				<span class="field">{s.label}</span>
				<span>{s.value}</span>
			</li>
		{/each}
	</ul>
</div>

<style>
	.bar {
		position: relative;
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-1) var(--space-3);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-md);
		background: var(--color-bg);
		color: var(--color-text-muted);
	}
	.focused {
		border-color: var(--color-accent);
	}
	.tokens {
		display: contents;
		list-style: none;
	}
	.token {
		display: inline-flex;
		align-items: center;
		gap: var(--space-1);
		padding: 0 var(--space-1) 0 var(--space-2);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-pill);
		background: var(--color-surface);
		color: var(--color-text);
		font-size: var(--text-sm);
		white-space: nowrap;
	}
	.token button {
		display: inline-flex;
		padding: 2px;
		border: 0;
		border-radius: var(--radius-pill);
		background: transparent;
		color: var(--color-text-muted);
		cursor: pointer;
	}
	.token button:hover {
		background: var(--color-surface-raised);
		color: var(--color-text);
	}
	.field {
		font-weight: 600;
	}
	input {
		flex: 1;
		min-width: 10rem;
		padding: var(--space-1) 0;
		border: 0;
		outline: none;
		background: transparent;
		color: var(--color-text);
		font: inherit;
		font-size: var(--text-sm);
	}
	.suggestions {
		position: absolute;
		top: calc(100% + var(--space-1));
		left: 0;
		right: 0;
		z-index: 10;
		margin: 0;
		padding: var(--space-1) 0;
		list-style: none;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-md);
		background: var(--color-surface-raised);
		color: var(--color-text);
		font-size: var(--text-sm);
	}
	.suggestions li {
		display: flex;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-3);
		cursor: pointer;
	}
	.suggestions .active {
		background: var(--color-surface);
	}
</style>
