<script lang="ts">
	import { Input, Text } from '#lib/components/atoms/index.js';
	import { FormField } from '#lib/components/molecules/index.js';
	import { setupToken } from '#lib/api.js';
	import { MIN_PASSWORD, passwordProblem, type SetupDraft } from '#lib/setup.js';

	interface Props {
		draft: SetupDraft;
		/** Whether the token came from the setup URL (then it is not asked for). */
		tokenFromURL: boolean;
	}
	let { draft, tokenFromURL }: Props = $props();
	let problem = $state<string | null>(null);

	export function validate(): Promise<boolean> {
		problem = passwordProblem(draft);
		if (!problem) setupToken.set(draft.token.trim());
		return Promise.resolve(!problem);
	}
</script>

<Text>Welcome to arrd. Choose the password you will use to sign in to this web interface.</Text>
{#if !tokenFromURL}
	<FormField id="token" label="Setup token" hint="Printed in the arrd log when it starts for the first time">
		{#snippet children(a)}<Input {...a} bind:value={draft.token} autocomplete="off" monospace />{/snippet}
	</FormField>
{/if}
<FormField id="password" label="Password" hint="At least {MIN_PASSWORD} characters">
	{#snippet children(a)}<Input {...a} type="password" autocomplete="new-password" bind:value={draft.password} />{/snippet}
</FormField>
<FormField id="password2" label="Confirm password">
	{#snippet children(a)}<Input {...a} type="password" autocomplete="new-password" bind:value={draft.passwordConfirm} />{/snippet}
</FormField>
{#if problem}<Text tone="danger" size="sm">{problem}</Text>{/if}
