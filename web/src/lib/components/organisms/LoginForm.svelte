<script lang="ts">
	import { goto } from '$app/navigation';
	import { Button, Input } from '#lib/components/atoms/index.js';
	import { ErrorMessage, FormField } from '#lib/components/molecules/index.js';
	import { HOME, safeNext } from '#lib/nav/auth-gate.js';
	import { useMutation } from '#lib/api/query.js';

	let { next = null }: { next?: string | null } = $props();
	let password = $state('');
	const login = useMutation('login');

	function submit(e: SubmitEvent) {
		e.preventDefault();
		login.mutate(password, { onSuccess: () => void goto(safeNext(next) ?? HOME) });
	}
</script>

<form class="form" onsubmit={submit}>
	<FormField id="password" label="Password">
		{#snippet children(a)}
			<Input type="password" autocomplete="current-password" required bind:value={password} {...a} />
		{/snippet}
	</FormField>
	{#if login.error}<ErrorMessage error={login.error} />{/if}
	<Button type="submit" variant="primary" loading={login.isPending}>Sign in</Button>
</form>

<style>
	.form {
		display: grid;
		gap: var(--space-4);
	}
</style>
