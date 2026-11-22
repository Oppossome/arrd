<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { goto } from '$app/navigation';
	import { Text } from '#lib/components/atoms/index.js';
	import { ErrorMessage } from '#lib/components/molecules/index.js';
	import { setupToken } from '#lib/api.js';
	import { HOME } from '#lib/auth-gate.js';
	import { queries, useMutation } from '#lib/query.js';
	import { toSetupInput, type SetupDraft } from '#lib/setup.js';
	import { toasts } from '#lib/toast.svelte.js';

	let { draft }: { draft: SetupDraft } = $props();
	const complete = useMutation('completeSetup');
	const input = $derived(toSetupInput(draft));
	const definitions = createQuery(() => queries.indexerDefinitions());
	const siteName = (id = '') => definitions.data?.find((d) => d.id === id)?.name ?? id;

	export async function validate(): Promise<boolean> {
		try {
			const res = await complete.mutateAsync(input);
			setupToken.clear();
			for (const w of res.warnings ?? []) toasts.push('info', w, 10_000);
			if (res.runtime_error) toasts.push('error', `Setup saved, but downloads could not start: ${res.runtime_error}`, 0);
			await goto(HOME);
			return true;
		} catch {
			return false;
		}
	}
</script>

<Text>Check your choices. Everything can be changed later in Settings.</Text>
<dl class="summary">
	<dt>TMDB key</dt><dd>{draft.tmdbKey ? 'Set' : 'Missing'}</dd>
	<dt>Incomplete downloads</dt><dd class="mono">{input.downloads.incomplete_dir}</dd>
	<dt>Completed downloads</dt><dd class="mono">{input.downloads.complete_dir}</dd>
	<dt>Movie library</dt><dd class="mono">{input.movie_root ?? '—'}</dd>
	<dt>TV library</dt><dd class="mono">{input.tv_root ?? '—'}</dd>
	<dt>Import mode</dt><dd>{input.downloads.import_mode}</dd>
	<dt>VPN</dt><dd>{input.vpn ? `${input.vpn.provider} (traffic leaves from ${draft.vpnTestedIP})` : 'None'}</dd>
	<dt>Indexers and downloads use</dt><dd>{input.vpn ? 'Only the VPN' : 'Your normal connection'}</dd>
	<dt>Indexers</dt><dd>{input.indexers?.length ? input.indexers.map((i) => siteName(i.definition)).join(', ') : 'None'}</dd>
</dl>
{#if complete.error}<ErrorMessage error={complete.error} />{/if}

<style>
	.summary {
		display: grid;
		grid-template-columns: auto 1fr;
		gap: var(--space-2) var(--space-4);
		margin: 0;
		font-size: var(--text-sm);
	}
	dt {
		color: var(--color-text-muted);
	}
	dd {
		margin: 0;
	}
	.mono {
		font-family: var(--font-mono);
		word-break: break-all;
	}
</style>
