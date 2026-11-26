<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { Badge, Button, IconButton, Spinner, Text, Toggle } from '#lib/components/atoms/index.js';
	import { ConfirmDialog, EmptyState, ErrorMessage, MediaServerForm } from '#lib/components/molecules/index.js';
	import { timeAgo } from '#lib/utils/format.js';
	import { queries, useMutation } from '#lib/api/query.js';
	import { toasts } from '#lib/stores/toast.svelte.js';
	import type { MediaServer, MediaServerInput } from '#lib/api/types.js';

	const servers = createQuery(() => queries.mediaServers());
	const definitions = createQuery(() => queries.mediaServerDefinitions());
	const save = useMutation('saveMediaServer');
	const toggle = useMutation('saveMediaServer');
	const del = useMutation('deleteMediaServer');
	const test = useMutation('testMediaServer');

	const typeName = (id: string) => definitions.data?.find((d) => d.id === id)?.name ?? id;

	/** The server whose form is open: an id, 'new', or null. */
	let editing = $state<number | 'new' | null>(null);
	let toggling = $state<number | null>(null);
	let testing = $state<number | null>(null);
	let deleting = $state<MediaServer | null>(null);

	function open(target: number | 'new') {
		save.reset();
		editing = editing === target ? null : target;
	}

	function submit(input: MediaServerInput, server?: MediaServer) {
		save.mutate(server ? { id: server.id, input } : { input }, {
			onSuccess: (saved) => {
				editing = null;
				toasts.push('success', `${saved.name} ${server ? 'saved' : 'added'}`);
			}
		});
	}

	function setEnabled(server: MediaServer, enabled: boolean) {
		toggle.reset();
		toggling = server.id;
		toggle.mutate(
			{ id: server.id, input: { enabled } },
			{
				onSuccess: () => toasts.push('success', `${server.name} switched ${enabled ? 'on' : 'off'}`),
				onSettled: () => (toggling = null)
			}
		);
	}

	function runTest(server: MediaServer) {
		testing = server.id;
		test.mutate(server.id, {
			onSuccess: () => toasts.push('success', `${server.name} works`),
			onError: (err) => toasts.push('error', `${server.name}: ${err instanceof Error ? err.message : String(err)}`),
			onSettled: () => (testing = null)
		});
	}
</script>

<section class="panel" aria-label="Media servers">
	<Text size="sm" tone="muted">
		Media servers are asked to rescan their libraries shortly after downloads are imported, so new movies and episodes show up
		without waiting for their scheduled scan.
	</Text>

	{#if servers.isPending || definitions.isPending}
		<Spinner />
	{:else if servers.isError}
		<ErrorMessage error={servers.error} />
	{:else if definitions.isError}
		<ErrorMessage error={definitions.error} />
	{:else}
		{#if toggle.error}<ErrorMessage error={toggle.error} />{/if}
		<ul class="list">
			{#each servers.data as server (server.id)}
				<li class="item">
					<div class="row">
						<Toggle
							checked={server.enabled}
							label="Switch {server.enabled ? 'off' : 'on'} {server.name}"
							disabled={toggling === server.id}
							onchange={(on) => setEnabled(server, on)}
						/>
						<Text as="span" size="sm">{server.name}</Text>
						<Text as="span" size="xs" tone="muted">{typeName(server.definition)}</Text>
						{#if !server.enabled}<Badge>Off</Badge>{/if}
						<span class="spacer"></span>
						<Button size="sm" variant="ghost" loading={testing === server.id} onclick={() => runTest(server)}>Test</Button>
						<IconButton icon="settings" label="Edit {server.name}" onclick={() => open(server.id)} />
						<IconButton icon="trash" tone="danger" label="Delete {server.name}" onclick={() => (deleting = server)} />
					</div>
					{#if server.last_error}
						<Text size="xs" tone="danger">Last error {timeAgo(server.last_error_at)}: {server.last_error}</Text>
					{/if}
					{#if editing === server.id}
						<MediaServerForm
							definitions={definitions.data}
							{server}
							busy={save.isPending}
							error={save.error}
							onsubmit={(input) => submit(input, server)}
							oncancel={() => (editing = null)}
						/>
					{/if}
				</li>
			{:else}
				<li><EmptyState message="No media servers yet." /></li>
			{/each}
		</ul>

		{#if editing === 'new'}
			<MediaServerForm
				definitions={definitions.data}
				busy={save.isPending}
				error={save.error}
				onsubmit={(input) => submit(input)}
				oncancel={() => (editing = null)}
			/>
		{:else if definitions.data.length > 0}
			<div class="actions"><Button onclick={() => open('new')}>Add media server</Button></div>
		{/if}
	{/if}
</section>

<ConfirmDialog
	open={deleting !== null}
	title="Delete {deleting?.name}?"
	confirmLabel="Delete"
	danger
	busy={del.isPending}
	onconfirm={() => deleting && del.mutate(deleting.id, { onSuccess: () => (deleting = null) })}
	oncancel={() => (deleting = null)}
>
	<Text size="sm">It will no longer be told about imports. Nothing on the media server changes.</Text>
	{#if del.error}<ErrorMessage error={del.error} />{/if}
</ConfirmDialog>

<style>
	.panel {
		display: grid;
		gap: var(--space-4);
	}
	.list {
		display: grid;
		gap: var(--space-2);
		margin: 0;
		padding: 0;
		list-style: none;
	}
	.item {
		display: grid;
		gap: var(--space-3);
		padding: var(--space-2) var(--space-3);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-md);
	}
	.row {
		display: flex;
		align-items: center;
		gap: var(--space-3);
	}
	.spacer {
		flex: 1;
	}
	.actions {
		display: flex;
		justify-content: flex-end;
	}
</style>
