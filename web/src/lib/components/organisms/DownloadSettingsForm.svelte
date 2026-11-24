<script lang="ts">
	import { createQuery, useQueryClient } from '@tanstack/svelte-query';
	import { Button, Select, Spinner, Text } from '#lib/components/atoms/index.js';
	import { DirInput, ErrorMessage, FormField } from '#lib/components/molecules/index.js';
	import { fieldError } from '#lib/api.js';
	import { queries, useMutation } from '#lib/query.js';
	import { toasts } from '#lib/toast.svelte.js';
	import type { DownloadSettings, ImportMode } from '#lib/types.js';

	const qc = useQueryClient();
	const settings = createQuery(() => queries.downloadSettings());
	const vpn = createQuery(() => queries.vpnSettings(false));
	const update = useMutation('updateDownloads');
	const listDirs = (p: string) => qc.fetchQuery(queries.dirs(p));

	let draft = $state<DownloadSettings | null>(null);
	$effect(() => {
		if (settings.data && !draft) draft = { ...settings.data };
	});
	let warnings = $state<string[]>([]);

	const fieldErr = (f: string) => fieldError(update.error, f);

	function save(e: SubmitEvent) {
		e.preventDefault();
		if (!draft) return;
		update.mutate(draft, {
			onSuccess: (r) => {
				warnings = r.warnings ?? [];
				toasts.push('success', 'Download settings saved');
			}
		});
	}
</script>

{#if settings.isPending || !draft}
	<Spinner label="Loading settings" />
{:else if settings.isError}
	<ErrorMessage error={settings.error} />
{:else}
	<form class="form" onsubmit={save}>
		<FormField id="incomplete" label="Incomplete downloads" hint="Sources download here" error={fieldErr('incomplete_dir')}>
			{#snippet children(a)}<DirInput id={a.id} bind:value={draft!.incomplete_dir} {listDirs} invalid={a.invalid} />{/snippet}
		</FormField>
		<FormField id="complete" label="Completed downloads" hint="Finished sources are moved here and keep seeding" error={fieldErr('complete_dir')}>
			{#snippet children(a)}<DirInput id={a.id} bind:value={draft!.complete_dir} {listDirs} invalid={a.invalid} />{/snippet}
		</FormField>
		<div class="network">
			<Text size="sm" tone="muted" as="span">Network</Text>
			{#if vpn.data?.configured}
				<Text size="sm" as="span">Through the VPN. Downloads stop while the tunnel is down.</Text>
			{:else}
				<Text size="sm" as="span">Direct (no VPN configured). Set up a VPN to route downloads through it.</Text>
			{/if}
		</div>
		<FormField id="import" label="Import mode" error={fieldErr('import_mode')}>
			{#snippet children(a)}
				<Select
					id={a.id}
					bind:value={draft!.import_mode}
					options={[
						{ value: 'hardlink' as ImportMode, label: 'Hardlink (no extra space, keeps seeding)' },
						{ value: 'copy' as ImportMode, label: 'Copy' },
						{ value: 'move' as ImportMode, label: 'Move (stops seeding)' }
					]}
				/>
			{/snippet}
		</FormField>
		{#each warnings as w (w)}<Text size="sm" tone="warning">{w}</Text>{/each}
		{#if update.error && !['incomplete_dir', 'complete_dir', 'import_mode'].some(fieldErr)}
			<ErrorMessage error={update.error} />
		{/if}
		<div><Button type="submit" variant="primary" loading={update.isPending}>Save</Button></div>
	</form>
{/if}

<style>
	.form {
		display: grid;
		gap: var(--space-4);
	}
	.network {
		display: grid;
		gap: var(--space-1);
	}
</style>
