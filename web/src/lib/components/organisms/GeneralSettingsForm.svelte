<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { Button, Heading, Input, Text } from '#lib/components/atoms/index.js';
	import { CopyField, ErrorMessage, FormField } from '#lib/components/molecules/index.js';
	import { fieldError } from '#lib/api.js';
	import { queries, useMutation } from '#lib/query.js';
	import { toasts } from '#lib/toast.svelte.js';

	const general = createQuery(() => queries.generalSettings());
	const updateGeneral = useMutation('updateGeneral');
	const regenerate = useMutation('regenerateAPIKey');
	const changePassword = useMutation('changePassword');

	let tmdbKey = $state('');
	let newKey = $state<string | null>(null);
	let current = $state('');
	let next = $state('');
	let confirm = $state('');
	const mismatch = $derived(confirm.length > 0 && next !== confirm);

	function saveTMDB(e: SubmitEvent) {
		e.preventDefault();
		updateGeneral.mutate(tmdbKey, {
			onSuccess: () => {
				tmdbKey = '';
				toasts.push('success', 'TMDB key saved');
			}
		});
	}
	function savePassword(e: SubmitEvent) {
		e.preventDefault();
		if (mismatch) return;
		changePassword.mutate(
			{ current, next },
			{
				onSuccess: () => {
					current = next = confirm = '';
					toasts.push('success', 'Password changed; other sessions were signed out');
				}
			}
		);
	}
	const fieldErr = fieldError;
</script>

<section class="block">
	<Heading level={2}>TMDB</Heading>
	<form class="row" onsubmit={saveTMDB}>
		<FormField
			id="tmdb"
			label="API key"
			hint={general.data?.tmdb_api_key_set ? 'A key is saved. Enter a new one to replace it.' : 'Required to look up movies and series.'}
			error={fieldErr(updateGeneral.error, 'tmdb_api_key')}
		>
			{#snippet children(a)}<Input {...a} type="password" autocomplete="off" bind:value={tmdbKey} />{/snippet}
		</FormField>
		<Button type="submit" variant="primary" disabled={!tmdbKey} loading={updateGeneral.isPending}>Save</Button>
	</form>
</section>

<section class="block">
	<Heading level={2}>API key</Heading>
	<Text size="sm" tone="muted">For scripts and other apps. Regenerating it stops the old key working immediately.</Text>
	{#if newKey}
		<CopyField value={newKey} label="new API key" />
		<Text size="xs" tone="warning">Copy it now; it will not be shown again.</Text>
	{/if}
	<div>
		<Button loading={regenerate.isPending} onclick={() => regenerate.mutate(undefined, { onSuccess: (r) => (newKey = r.api_key) })}>
			Regenerate API key
		</Button>
	</div>
</section>

<section class="block">
	<Heading level={2}>Password</Heading>
	<form class="stack" onsubmit={savePassword}>
		<FormField id="pw-current" label="Current password" error={fieldErr(changePassword.error, 'current_password')}>
			{#snippet children(a)}<Input {...a} type="password" autocomplete="current-password" bind:value={current} required />{/snippet}
		</FormField>
		<FormField id="pw-new" label="New password" hint="At least 8 characters" error={fieldErr(changePassword.error, 'new_password')}>
			{#snippet children(a)}<Input {...a} type="password" autocomplete="new-password" bind:value={next} required minlength={8} />{/snippet}
		</FormField>
		<FormField id="pw-confirm" label="Confirm new password" error={mismatch ? 'Passwords do not match' : undefined}>
			{#snippet children(a)}<Input {...a} type="password" autocomplete="new-password" bind:value={confirm} required />{/snippet}
		</FormField>
		{#if changePassword.error && !fieldErr(changePassword.error, 'current_password') && !fieldErr(changePassword.error, 'new_password')}
			<ErrorMessage error={changePassword.error} />
		{/if}
		<div><Button type="submit" variant="primary" disabled={mismatch} loading={changePassword.isPending}>Change password</Button></div>
	</form>
</section>

<style>
	.block,
	.stack {
		display: grid;
		gap: var(--space-3);
	}
	.row {
		display: grid;
		grid-template-columns: 1fr auto;
		align-items: end;
		gap: var(--space-3);
	}
</style>
