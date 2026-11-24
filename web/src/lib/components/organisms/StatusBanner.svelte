<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { Icon, Link, Text } from '#lib/components/atoms/index.js';
	import { queries } from '#lib/api/query.js';

	const status = createQuery(() => queries.status());
</script>

{#if status.data && !status.data.downloads_available}
	<div class="banner" role="alert">
		<Icon name="alert" />
		<Text size="sm" as="span">Downloads are paused: {status.data.downloads_error ?? 'unknown reason'}</Text>
		<Link href="/settings?tab=vpn">VPN settings</Link>
	</div>
{/if}

<style>
	.banner {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-2) var(--space-5);
		border-bottom: 1px solid var(--color-danger);
		background: var(--color-surface);
		color: var(--color-danger);
	}
</style>
