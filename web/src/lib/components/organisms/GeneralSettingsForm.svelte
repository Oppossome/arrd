<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { Button, Heading, Input, Spinner, Text } from '#lib/components/atoms/index.js';
	import { ConfirmDialog, CopyField, ErrorMessage, FormField, SecretField } from '#lib/components/molecules/index.js';
	import { fieldError } from '#lib/api/client.js';
	import { queries, useMutation } from '#lib/api/query.js';
	import { toasts } from '#lib/stores/toast.svelte.js';

	const general = createQuery(() => queries.generalSettings());
	const updateGeneral = useMutation('updateGeneral');
	const regenerate = useMutation('regenerateAPIKey');
	const changePassword = useMutation('changePassword');

	const tmdbSet = $derived(general.data?.tmdb_api_key_set ?? false);
	let editingTMDB = $state(false);
	let tmdbKey = $state('');

	let confirmRegenerate = $state(false);
	let newKey = $state<string | null>(null);

	let changingPassword = $state(false);
	let current = $state('');
	let next = $state('');
	let confirm = $state('');
	const mismatch = $derived(confirm.length > 0 && next !== confirm);

	function closeTMDB() {
		editingTMDB = false;
		tmdbKey = '';
		updateGeneral.reset();
	}
	function saveTMDB(e: SubmitEvent) {
		e.preventDefault();
		updateGeneral.mutate(tmdbKey, {
			onSuccess: () => {
				closeTMDB();
				toasts.push('success', 'TMDB key saved');
			}
		});
	}

	function doRegenerate() {
		regenerate.mutate(undefined, {
			onSuccess: (r) => {
				newKey = r.api_key;
				confirmRegenerate = false;
			}
		});
	}
	function cancelRegenerate() {
		confirmRegenerate = false;
		regenerate.reset();
	}

	function closePassword() {
		changingPassword = false;
		current = next = confirm = '';
		changePassword.reset();
	}
	function savePassword(e: SubmitEvent) {
		e.preventDefault();
		if (mismatch) return;
		changePassword.mutate(
			{ current, next },
			{
				onSuccess: () => {
					closePassword();
					toasts.push('success', 'Password changed; other sessions were signed out');
				}
			}
		);
	}
	const currentPwErr = $derived(fieldError(changePassword.error, 'current_password'));
	const newPwErr = $derived(fieldError(changePassword.error, 'new_password'));
</script>

{#if general.isPending}
	<Spinner label="Loading settings" />
{:else if general.isError}
	<ErrorMessage error={general.error} />
{:else}
	<section class="section" aria-label="TMDB">
		<div class="head">
			<Heading level={2}>TMDB</Heading>
			<Text size="sm" tone="muted">Used to look up movies and series.</Text>
		</div>
		{#if editingTMDB}
			<form class="stack" onsubmit={saveTMDB}>
				<FormField
					id="tmdb"
					label="TMDB API key"
					hint={tmdbSet ? 'Replaces the saved key.' : 'Find it in your TMDB account settings.'}
					error={fieldError(updateGeneral.error, 'tmdb_api_key')}
				>
					{#snippet children(a)}<Input {...a} type="password" autocomplete="off" bind:value={tmdbKey} />{/snippet}
				</FormField>
				<div class="actions">
					<Button onclick={closeTMDB} disabled={updateGeneral.isPending}>Cancel</Button>
					<Button type="submit" variant="primary" disabled={!tmdbKey} loading={updateGeneral.isPending}>Save</Button>
				</div>
			</form>
		{:else}
			<SecretField label="TMDB API key" hint={general.data?.tmdb_api_key_hint}>
				{#snippet actions()}
					<Button variant={tmdbSet ? 'secondary' : 'primary'} onclick={() => (editingTMDB = true)}>
						{tmdbSet ? 'Replace' : 'Add key'}
					</Button>
				{/snippet}
			</SecretField>
		{/if}
	</section>

	<section class="section" aria-label="App API key">
		<div class="head">
			<Heading level={2}>App API key</Heading>
			<Text size="sm" tone="muted">For scripts and other apps. Regenerating it stops the old key working immediately.</Text>
		</div>
		{#if newKey}
			<div class="reveal">
				<CopyField value={newKey} label="new API key" />
				<div class="reveal-foot">
					<Text size="xs" tone="warning" as="span">Copy it now; it will not be shown again.</Text>
					<Button size="sm" onclick={() => (newKey = null)}>Done</Button>
				</div>
			</div>
		{:else}
			<SecretField label="App API key" hint={general.data?.api_key_hint}>
				{#snippet actions()}
					<Button onclick={() => (confirmRegenerate = true)}>Regenerate</Button>
				{/snippet}
			</SecretField>
		{/if}
	</section>
{/if}

<section class="section" aria-label="Password">
	<div class="head">
		<Heading level={2}>Password</Heading>
		<Text size="sm" tone="muted">Changing it signs out your other sessions.</Text>
	</div>
	{#if changingPassword}
		<form class="stack" onsubmit={savePassword}>
			<FormField id="pw-current" label="Current password" error={currentPwErr}>
				{#snippet children(a)}<Input {...a} type="password" autocomplete="current-password" bind:value={current} required />{/snippet}
			</FormField>
			<div class="pair">
				<FormField id="pw-new" label="New password" hint="At least 8 characters" error={newPwErr}>
					{#snippet children(a)}<Input {...a} type="password" autocomplete="new-password" bind:value={next} required minlength={8} />{/snippet}
				</FormField>
				<FormField id="pw-confirm" label="Confirm new password" error={mismatch ? 'Passwords do not match' : undefined}>
					{#snippet children(a)}<Input {...a} type="password" autocomplete="new-password" bind:value={confirm} required />{/snippet}
				</FormField>
			</div>
			{#if changePassword.error && !currentPwErr && !newPwErr}
				<ErrorMessage error={changePassword.error} />
			{/if}
			<div class="actions">
				<Button onclick={closePassword} disabled={changePassword.isPending}>Cancel</Button>
				<Button type="submit" variant="primary" disabled={mismatch} loading={changePassword.isPending}>Change password</Button>
			</div>
		</form>
	{:else}
		<div><Button onclick={() => (changingPassword = true)}>Change password…</Button></div>
	{/if}
</section>

<ConfirmDialog
	open={confirmRegenerate}
	title="Regenerate the API key?"
	confirmLabel="Regenerate"
	danger
	busy={regenerate.isPending}
	onconfirm={doRegenerate}
	oncancel={cancelRegenerate}
>
	<Text size="sm" tone="warning">Scripts and apps using the current key will stop working until you give them the new one.</Text>
	{#if regenerate.error}<ErrorMessage error={regenerate.error} />{/if}
</ConfirmDialog>

<style>
	.section {
		display: grid;
		gap: var(--space-4);
	}
	.head {
		display: grid;
		gap: var(--space-1);
	}
	.stack {
		display: grid;
		gap: var(--space-3);
	}
	.pair {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(14rem, 1fr));
		align-items: start;
		gap: var(--space-3);
	}
	.actions {
		display: flex;
		justify-content: flex-end;
		gap: var(--space-2);
	}
	.reveal {
		display: grid;
		gap: var(--space-2);
		padding: var(--space-3);
		border: 1px solid var(--color-warning);
		border-radius: var(--radius-md);
	}
	.reveal-foot {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-2);
	}
</style>
