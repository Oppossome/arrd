<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import {
		DownloadSettingsForm,
		GeneralSettingsForm,
		IndexerDefinitionsPanel,
		IndexersPanel,
		QualitySettingsForm,
		RootFoldersPanel,
		VPNPanel
	} from '#lib/components/organisms/index.js';
	import { SettingsLayout } from '#lib/components/templates/index.js';

	const tabs = [
		{ key: 'general', label: 'General' },
		{ key: 'downloads', label: 'Downloads' },
		{ key: 'quality', label: 'Quality' },
		{ key: 'folders', label: 'Folders' },
		{ key: 'indexers', label: 'Indexers' },
		{ key: 'vpn', label: 'VPN' }
	] as const;
	type Tab = (typeof tabs)[number]['key'];

	const active = $derived.by((): Tab => {
		const t = page.url.searchParams.get('tab');
		return tabs.some((x) => x.key === t) ? (t as Tab) : 'general';
	});
</script>

<SettingsLayout tabs={[...tabs]} {active} onselect={(k) => goto(`?tab=${k}`, { replace: true })}>
	{#if active === 'general'}<GeneralSettingsForm />
	{:else if active === 'downloads'}<DownloadSettingsForm />
	{:else if active === 'quality'}<QualitySettingsForm />
	{:else if active === 'folders'}<RootFoldersPanel />
	{:else if active === 'indexers'}
		<IndexerDefinitionsPanel />
		<IndexersPanel />
	{:else}<VPNPanel />{/if}
</SettingsLayout>
