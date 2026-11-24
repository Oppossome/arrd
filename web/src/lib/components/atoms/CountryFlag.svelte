<script lang="ts">
	import { countryName, flagEmoji } from '#lib/utils/countries.js';

	interface Props {
		/** ISO 3166-1 alpha-2; empty when unknown. */
		code: string;
		/** Also show the code next to the flag. */
		showCode?: boolean;
	}
	let { code, showCode = false }: Props = $props();
	const flag = $derived(flagEmoji(code));
	const name = $derived(countryName(code));
</script>

<span class="flag" title={name}>
	<span role="img" aria-label={name}>{flag || '🌐'}</span>
	{#if showCode}<span class="code" aria-hidden="true">{code ? code.toUpperCase() : '—'}</span>{/if}
</span>

<style>
	.flag {
		display: inline-flex;
		align-items: center;
		gap: var(--space-1);
		white-space: nowrap;
	}
	.code {
		color: var(--color-text-muted);
		font-family: var(--font-mono);
		font-size: var(--text-xs);
	}
</style>
