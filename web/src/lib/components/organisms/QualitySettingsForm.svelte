<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { Button, Input, Select, Spinner } from '#lib/components/atoms/index.js';
	import { ErrorMessage, FormField } from '#lib/components/molecules/index.js';
	import { fieldError } from '#lib/api/client.js';
	import { queries, useMutation } from '#lib/api/query.js';
	import { toasts } from '#lib/stores/toast.svelte.js';
	import type { QualitySettings } from '#lib/api/types.js';

	const quality = createQuery(() => queries.qualitySettings());
	const update = useMutation('updateQuality');
	let draft = $state<QualitySettings | null>(null);
	$effect(() => {
		if (quality.data && !draft) draft = { ...quality.data };
	});
	const resolutions = [480, 720, 1080, 2160].map((v) => ({ value: v, label: `${v}p` }));
	const fieldErr = (f: string) => fieldError(update.error, f);

	function save(e: SubmitEvent) {
		e.preventDefault();
		if (draft) update.mutate({ ...draft, max_size_mb: Number(draft.max_size_mb) }, { onSuccess: () => toasts.push('success', 'Quality limits saved') });
	}
</script>

{#if quality.isPending || !draft}
	<Spinner label="Loading settings" />
{:else if quality.isError}
	<ErrorMessage error={quality.error} />
{:else}
	<form class="form" onsubmit={save}>
		<div class="row">
			<FormField id="min-res" label="Minimum resolution" error={fieldErr('min_resolution')}>
				{#snippet children(a)}<Select id={a.id} bind:value={draft!.min_resolution} options={resolutions} />{/snippet}
			</FormField>
			<FormField id="max-res" label="Maximum resolution" error={fieldErr('max_resolution')}>
				{#snippet children(a)}<Select id={a.id} bind:value={draft!.max_resolution} options={resolutions} />{/snippet}
			</FormField>
		</div>
		<FormField id="max-size" label="Maximum size (MB per movie or episode)" hint="0 means unlimited" error={fieldErr('max_size_mb')}>
			{#snippet children(a)}<Input {...a} type="number" min="0" bind:value={draft!.max_size_mb} />{/snippet}
		</FormField>
		{#if update.error && !fieldErr('min_resolution') && !fieldErr('max_resolution') && !fieldErr('max_size_mb')}
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
	.row {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
		gap: var(--space-3);
	}
</style>
