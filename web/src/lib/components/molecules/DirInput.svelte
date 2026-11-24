<script lang="ts">
	import { Icon, IconButton, Input, Spinner, Text } from '#lib/components/atoms/index.js';
	import type { DirListing } from '#lib/api/types.js';

	interface Props {
		id?: string;
		value: string;
		/** Lists subdirectories; injected so this molecule stays API-free. */
		listDirs: (path: string) => Promise<DirListing>;
		invalid?: boolean;
		placeholder?: string;
		'aria-describedby'?: string;
	}
	let { id, value = $bindable(''), listDirs, invalid = false, placeholder = '/path/to/folder', ...rest }: Props =
		$props();

	let open = $state(false);
	let loading = $state(false);
	let listing = $state<DirListing | null>(null);
	let error = $state('');

	/** The parent of an absolute path ("/a/b" → "/a"), or null at the root. */
	const parentOf = (p: string): string | null => {
		const trimmed = p.replace(/\/+$/, '');
		if (!trimmed) return null;
		const i = trimmed.lastIndexOf('/');
		return i <= 0 ? '/' : trimmed.slice(0, i);
	};

	// A typed or suggested folder may not exist yet (it can be created
	// later), so browse the nearest existing parent instead of failing.
	async function browse(path: string) {
		loading = true;
		error = '';
		let target: string | null = path.startsWith('/') ? path : '/';
		try {
			for (;;) {
				try {
					listing = await listDirs(target);
					open = true;
					return;
				} catch (e) {
					target = parentOf(target);
					if (target === null) throw e;
				}
			}
		} catch (e) {
			error = e instanceof Error ? e.message : String(e);
		} finally {
			loading = false;
		}
	}

	function choose(path: string) {
		value = path;
		void browse(path);
	}

	const join = (base: string, name: string) => (base.endsWith('/') ? base + name : `${base}/${name}`);
</script>

<div class="dir-input">
	<div class="row">
		<Input {id} bind:value {invalid} {placeholder} monospace {...rest} />
		<IconButton icon="folder" label="Browse folders" onclick={() => (open ? (open = false) : browse(value))} />
	</div>
	{#if loading}<Spinner label="Loading folders" />{/if}
	{#if error}<Text tone="danger" size="xs">{error}</Text>{/if}
	{#if open && listing}
		<Text size="xs" tone="muted" mono>{listing.path}</Text>
		<ul class="list" aria-label="Folders in {listing.path}">
			{#if listing.parent}
				<li><button type="button" onclick={() => choose(listing!.parent!)}><Icon name="up" /> ..</button></li>
			{/if}
			{#each listing.dirs as dir (dir)}
				<li>
					<button type="button" onclick={() => choose(join(listing!.path, dir))}><Icon name="folder" /> {dir}</button>
				</li>
			{:else}
				<li><Text size="xs" tone="muted">No subfolders</Text></li>
			{/each}
		</ul>
	{/if}
</div>

<style>
	.dir-input {
		display: grid;
		gap: var(--space-2);
	}
	.row {
		display: flex;
		gap: var(--space-2);
		align-items: center;
	}
	.list {
		list-style: none;
		margin: 0;
		padding: var(--space-1);
		max-height: 14rem;
		overflow: auto;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-md);
		background: var(--color-surface);
	}
	.list button {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		width: 100%;
		padding: var(--space-1) var(--space-2);
		border: 0;
		border-radius: var(--radius-sm);
		background: transparent;
		color: var(--color-text);
		font: inherit;
		font-size: var(--text-sm);
		text-align: left;
		cursor: pointer;
	}
	.list button:hover {
		background: var(--color-surface-raised);
	}
</style>
