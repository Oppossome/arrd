<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { Sparkline, Text } from '#lib/components/atoms/index.js';
	import { formatRate } from '#lib/utils/format.js';
	import { queries } from '#lib/api/query.js';

	const throughput = createQuery(() => queries.throughput());
	const t = $derived(throughput.data);
	// Hidden until there is something to show.
	const visible = $derived(!!t && (t.active > 0 || t.history.some((s) => s.download_rate > 0 || s.upload_rate > 0)));
	const down = $derived(t?.history.map((s) => s.download_rate) ?? []);
</script>

{#if visible && t}
	<div class="widget" aria-label="Transfer rates">
		<div class="rates">
			<Text size="xs" as="span">↓ {formatRate(t.download_rate)}</Text>
			<Text size="xs" tone="muted" as="span">↑ {formatRate(t.upload_rate)}</Text>
		</div>
		<Sparkline values={down} max={t.peak_download_rate} label="Download rate, last 5 minutes" />
		<Text size="xs" tone="muted">Peak {formatRate(t.peak_download_rate)}</Text>
	</div>
{/if}

<style>
	.widget {
		display: grid;
		gap: var(--space-1);
		padding: var(--space-2) var(--space-3);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-md);
	}
	.rates {
		display: flex;
		justify-content: space-between;
		gap: var(--space-2);
	}
</style>
