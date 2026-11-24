<script lang="ts">
	import { createQuery, useQueryClient } from '@tanstack/svelte-query';
	import { Badge, Button, Heading, IconButton, Spinner, Text } from '#lib/components/atoms/index.js';
	import { DirInput, ErrorMessage } from '#lib/components/molecules/index.js';
	import { queries, useMutation } from '#lib/api/query.js';
	import type { MediaType } from '#lib/api/types.js';

	const qc = useQueryClient();
	const movieRoots = createQuery(() => queries.rootFolders('movie'));
	const tvRoots = createQuery(() => queries.rootFolders('tv'));
	const add = useMutation('addRootFolder');
	const setDefault = useMutation('setDefaultRootFolder');
	const remove = useMutation('deleteRootFolder');
	const listDirs = (p: string) => qc.fetchQuery(queries.dirs(p));

	let newPath = $state<Record<MediaType, string>>({ movie: '', tv: '' });
	const sections = $derived([
		{ mt: 'movie' as MediaType, title: 'Movie folders', q: movieRoots },
		{ mt: 'tv' as MediaType, title: 'TV folders', q: tvRoots }
	]);
</script>

{#each sections as s (s.mt)}
	<section class="block">
		<Heading level={2}>{s.title}</Heading>
		{#if s.q.isPending}
			<Spinner />
		{:else if s.q.isError}
			<ErrorMessage error={s.q.error} />
		{:else}
			<ul class="list">
				{#each s.q.data as r (r.id)}
					<li>
						<Text mono size="sm" as="span">{r.path}</Text>
						{#if r.is_default}<Badge tone="success">Default</Badge>{:else}
							<Button size="sm" variant="ghost" onclick={() => setDefault.mutate(r.id)}>Make default</Button>
						{/if}
						<IconButton icon="trash" tone="danger" label="Remove {r.path}" onclick={() => remove.mutate(r.id)} />
					</li>
				{:else}
					<li><Text size="sm" tone="muted" as="span">None yet.</Text></li>
				{/each}
			</ul>
		{/if}
		<form
			class="add"
			onsubmit={(e) => {
				e.preventDefault();
				add.mutate({ mediaType: s.mt, path: newPath[s.mt] }, { onSuccess: () => (newPath[s.mt] = '') });
			}}
		>
			<DirInput bind:value={newPath[s.mt]} {listDirs} aria-describedby="root-help" />
			<Button type="submit" disabled={!newPath[s.mt]} loading={add.isPending}>Add</Button>
		</form>
	</section>
{/each}
<Text id="root-help" size="xs" tone="muted">Folders must exist and be writable. Removing a folder never deletes files; it is refused while a movie or series uses it.</Text>
{#if add.error || remove.error}<ErrorMessage error={add.error ?? remove.error} />{/if}

<style>
	.block {
		display: grid;
		gap: var(--space-3);
	}
	.list {
		display: grid;
		gap: var(--space-2);
		margin: 0;
		padding: 0;
		list-style: none;
	}
	.list li {
		display: flex;
		align-items: center;
		gap: var(--space-3);
	}
	.add {
		display: grid;
		grid-template-columns: 1fr auto;
		align-items: start;
		gap: var(--space-2);
	}
</style>
