<script lang="ts">
	import { Sidebar, StatusBanner } from '#lib/components/organisms/index.js';
	import { AppShell } from '#lib/components/templates/index.js';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();
</script>

<AppShell>
	{#snippet sidebar()}<Sidebar />{/snippet}
	{#snippet banner()}<StatusBanner />{/snippet}
	{@render children()}
</AppShell>
