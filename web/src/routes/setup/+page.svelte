<script lang="ts">
	import { page } from '$app/state';
	import {
		IndexerDefinitionsPanel,
		SetupFoldersStep,
		SetupIndexersStep,
		SetupPasswordStep,
		SetupReviewStep,
		SetupTMDBStep,
		SetupVPNStep
	} from '#lib/components/organisms/index.js';
	import { WizardLayout } from '#lib/components/templates/index.js';
	import { setupToken } from '#lib/api/client.js';
	import { newDraft, SETUP_STEPS } from '#lib/domain/setup.js';

	const urlToken = page.url.searchParams.get('token');
	const draft = $state(newDraft(urlToken ?? setupToken.get() ?? ''));
	let step = $state(0);
	let busy = $state(false);
	/** The current step's component; each exposes validate(). */
	let current = $state<{ validate: () => Promise<boolean> }>();

	async function next() {
		if (!current) return;
		busy = true;
		try {
			if ((await current.validate()) && step < SETUP_STEPS.length - 1) step++;
		} finally {
			busy = false;
		}
	}
</script>

<WizardLayout
	title="Set up arrd"
	steps={SETUP_STEPS}
	current={step}
	{busy}
	nextLabel={step === SETUP_STEPS.length - 1 ? 'Finish setup' : 'Continue'}
	onback={step > 0 ? () => step-- : undefined}
	onnext={next}
>
	{#if step === 0}<SetupPasswordStep bind:this={current} {draft} tokenFromURL={!!urlToken} />
	{:else if step === 1}<SetupTMDBStep bind:this={current} {draft} />
	{:else if step === 2}<SetupFoldersStep bind:this={current} {draft} />
	{:else if step === 3}<SetupVPNStep bind:this={current} {draft} />
	{:else if step === 4}
		<SetupIndexersStep bind:this={current} {draft} />
		<IndexerDefinitionsPanel />
	{:else}<SetupReviewStep bind:this={current} {draft} />{/if}
</WizardLayout>
