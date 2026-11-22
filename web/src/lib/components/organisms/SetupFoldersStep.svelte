<script lang="ts">
	import { untrack } from 'svelte';
	import { createQuery, useQueryClient } from '@tanstack/svelte-query';
	import { Checkbox, Select, Text } from '#lib/components/atoms/index.js';
	import { DirInput, ErrorMessage, FormField } from '#lib/components/molecules/index.js';
	import { queries, useMutation } from '#lib/query.js';
	import type { SetupDraft } from '#lib/setup.js';
	import type { ImportMode, PathsCheck } from '#lib/types.js';

	let { draft }: { draft: SetupDraft } = $props();
	const qc = useQueryClient();
	const check = useMutation('validatePaths');
	const listDirs = (p: string) => qc.fetchQuery({ ...queries.dirs(p), staleTime: 0 });
	let result = $state<PathsCheck | null>(null);

	// Pre-fill empty fields with the server's suggested folders.
	const status = createQuery(() => queries.setupStatus());
	$effect(() => {
		const s = status.data?.suggested;
		if (!s) return;
		untrack(() => {
			draft.incompleteDir ||= s.incomplete_dir;
			draft.completeDir ||= s.complete_dir;
			draft.movieRoot ||= s.movie_root;
			draft.tvRoot ||= s.tv_root;
		});
	});

	export async function validate(): Promise<boolean> {
		try {
			result = await check.mutateAsync({
				downloads: { incomplete_dir: draft.incompleteDir, complete_dir: draft.completeDir },
				movie_root: draft.movieRoot || undefined,
				tv_root: draft.tvRoot || undefined,
				create_missing: draft.createMissing
			});
			return result.valid;
		} catch {
			return false;
		}
	}
	const err = (f: string) => result?.errors[f];
</script>

<Text>Where should downloads go, and where is your library? Leave a library folder empty if you do not need it.</Text>
<FormField id="incomplete" label="Incomplete downloads" error={err('downloads.incomplete_dir')}>
	{#snippet children(a)}<DirInput id={a.id} bind:value={draft.incompleteDir} {listDirs} invalid={a.invalid} />{/snippet}
</FormField>
<FormField id="complete" label="Completed downloads" hint="Must be on the same disk as incomplete downloads" error={err('downloads.complete_dir')}>
	{#snippet children(a)}<DirInput id={a.id} bind:value={draft.completeDir} {listDirs} invalid={a.invalid} />{/snippet}
</FormField>
<FormField id="movie-root" label="Movie library" error={err('movie_root')}>
	{#snippet children(a)}<DirInput id={a.id} bind:value={draft.movieRoot} {listDirs} invalid={a.invalid} />{/snippet}
</FormField>
<FormField id="tv-root" label="TV library" error={err('tv_root')}>
	{#snippet children(a)}<DirInput id={a.id} bind:value={draft.tvRoot} {listDirs} invalid={a.invalid} />{/snippet}
</FormField>
<FormField id="import-mode" label="Import mode">
	{#snippet children(a)}
		<Select
			id={a.id}
			bind:value={draft.importMode}
			options={[
				{ value: 'hardlink' as ImportMode, label: 'Hardlink (no extra space, keeps seeding)' },
				{ value: 'copy' as ImportMode, label: 'Copy' },
				{ value: 'move' as ImportMode, label: 'Move (stops seeding)' }
			]}
		/>
	{/snippet}
</FormField>
<Checkbox bind:checked={draft.createMissing} label="Create folders that do not exist yet" />
{#each result?.warnings ?? [] as w (w)}<Text tone="warning" size="sm">{w}</Text>{/each}
{#if check.error}<ErrorMessage error={check.error} />{/if}
