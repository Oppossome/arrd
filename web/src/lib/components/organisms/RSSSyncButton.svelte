<script lang="ts">
	import { Button } from '#lib/components/atoms/index.js';
	import { useMutation } from '#lib/api/query.js';
	import { toasts } from '#lib/stores/toast.svelte.js';

	const sync = useMutation('rssSync');
</script>

<Button
	loading={sync.isPending}
	onclick={() =>
		sync.mutate(undefined, {
			onSuccess: (r) =>
				toasts.push(
					r.grabbed.length ? 'success' : 'info',
					`RSS: ${r.releases_seen} releases seen, ${r.grabbed.length} grabbed${r.indexer_failures.length ? `, ${r.indexer_failures.length} indexer(s) failed` : ''}`
				)
		})}>RSS sync now</Button
>
