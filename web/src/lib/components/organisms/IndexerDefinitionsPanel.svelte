<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { Button, Text } from '#lib/components/atoms/index.js';
	import { ErrorMessage } from '#lib/components/molecules/index.js';
	import { queries, useMutation } from '#lib/api/query.js';
	import { toasts } from '#lib/stores/toast.svelte.js';

	const definitions = createQuery(() => queries.indexerDefinitions());
	const upload = useMutation('uploadIndexerDefinitions');
	const reload = useMutation('reloadIndexerDefinitions');

	let picker = $state<HTMLInputElement>();
	/** Files skipped by the last upload or reload. */
	let skipped = $state<string[]>([]);

	function uploadFile(e: Event) {
		const input = e.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		input.value = ''; // picking the same file again uploads it again
		if (!file) return;
		upload.mutate(file, {
			onSuccess: (r) => {
				skipped = r.errors;
				const n = r.saved.length;
				const added = `Added ${n} site definition${n === 1 ? '' : 's'}`;
				if (r.errors.length) toasts.push('error', `${added}; ${r.errors.length} file(s) skipped`);
				else toasts.push('success', added);
			}
		});
	}

	function reloadDefinitions() {
		reload.mutate(undefined, {
			onSuccess: (r) => {
				skipped = r.errors;
				const sites = r.definitions.length;
				if (r.errors.length) toasts.push('error', `Loaded ${sites} site definitions; ${r.errors.length} could not be read`);
				else toasts.push('success', `Loaded ${sites} site definitions`);
			}
		});
	}
</script>

<section class="panel" aria-label="Site definitions">
	<Text size="sm" tone="muted">
		The sites arrd can search come from Cardigann site definitions. Upload a definition file (.yml), or a .zip of them,
		then switch on the sites you want. When a zip has versioned folders (definitions/v11, v12 …), each site comes from the newest version that can be read. An upload replaces any
		definition with the same id.
	</Text>
	<div class="toolbar">
		<input bind:this={picker} class="hidden" type="file" accept=".zip,.yml,.yaml" onchange={uploadFile} />
		<Button loading={upload.isPending} onclick={() => picker?.click()}>Upload definitions…</Button>
		<Button variant="ghost" loading={reload.isPending} onclick={reloadDefinitions}>Reload from disk</Button>
	</div>
	{#if upload.error}<ErrorMessage error={upload.error} />{/if}
	{#if reload.error}<ErrorMessage error={reload.error} />{/if}
	{#if skipped.length}
		<details>
			<summary><Text as="span" size="sm" tone="danger">{skipped.length} file(s) skipped</Text></summary>
			<ul class="skipped">
				{#each skipped as err (err)}<li><Text size="xs" tone="danger" mono>{err}</Text></li>{/each}
			</ul>
		</details>
	{/if}

	{#if definitions.data?.length}
		<Text size="sm" tone="muted">{definitions.data.length} site definitions loaded</Text>
	{/if}
</section>

<style>
	.panel {
		display: grid;
		gap: var(--space-3);
	}
	.toolbar {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-2);
	}
	.hidden {
		display: none;
	}
	summary {
		cursor: pointer;
	}
	.skipped {
		margin: var(--space-2) 0 0;
		padding-left: var(--space-4);
	}
</style>
