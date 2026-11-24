<script lang="ts">
	import { Input, Link, Text } from '#lib/components/atoms/index.js';
	import { FormField } from '#lib/components/molecules/index.js';
	import { useMutation } from '#lib/api/query.js';
	import type { SetupDraft } from '#lib/domain/setup.js';

	let { draft }: { draft: SetupDraft } = $props();
	const check = useMutation('validateTMDB');

	export async function validate(): Promise<boolean> {
		try {
			await check.mutateAsync(draft.tmdbKey.trim());
			return true;
		} catch {
			return false;
		}
	}
</script>

<Text>arrd uses The Movie Database (TMDB) for titles, episodes and posters. A free API key is enough.</Text>
<Text size="sm" tone="muted">Get one at <Link href="https://www.themoviedb.org/settings/api">themoviedb.org/settings/api</Link>; either the API key or the read access token works.</Text>
<FormField id="tmdb" label="TMDB API key" error={check.error?.message}>
	{#snippet children(a)}<Input {...a} bind:value={draft.tmdbKey} autocomplete="off" monospace />{/snippet}
</FormField>
{#if check.isSuccess}<Text tone="success" size="sm">Key accepted.</Text>{/if}
