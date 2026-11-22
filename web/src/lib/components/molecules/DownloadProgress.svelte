<script lang="ts">
	import { ProgressBar, Text } from '#lib/components/atoms/index.js';
	import { formatBytes, formatRate, percent } from '#lib/format.js';
	import type { Progress } from '#lib/types.js';

	let { progress }: { progress?: Progress } = $props();
	const pct = $derived(progress ? percent(progress.bytes_completed, progress.bytes_total) : 0);
</script>

<div class="progress">
	{#if !progress}
		<Text size="xs" tone="muted">Not running</Text>
	{:else if !progress.have_info}
		<ProgressBar value={0} label="Fetching metadata" />
		<Text size="xs" tone="muted">Fetching metadata… {progress.active_peers} peers</Text>
	{:else}
		<ProgressBar value={pct} label="Download progress" tone={progress.complete ? 'success' : 'info'} />
		<Text size="xs" tone="muted">
			{pct}% · {formatBytes(progress.bytes_completed)} / {formatBytes(progress.bytes_total)} · {progress.active_peers}
			peers ({progress.seeders} seeders){#if progress.download_rate > 0} · {formatRate(progress.download_rate)}{/if}
		</Text>
	{/if}
</div>

<style>
	.progress {
		display: grid;
		gap: var(--space-1);
		min-width: 12rem;
	}
</style>
