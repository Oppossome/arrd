<script lang="ts">
	import { SearchBox } from '#lib/components/molecules/index.js';
	import type { MediaKind } from '#lib/links.js';

	interface Props {
		kind: MediaKind;
		/** The debounced search: empty until the user types at least two characters. */
		query?: string;
	}
	let { kind, query = $bindable('') }: Props = $props();

	let search = $state('');
	$effect(() => {
		const v = search.trim();
		const t = setTimeout(() => (query = v.length >= 2 ? v : ''), 350);
		return () => clearTimeout(t);
	});
	const noun = $derived(kind === 'movies' ? 'movies' : 'shows');
</script>

<SearchBox bind:value={search} label="Search {noun}" placeholder="Search {noun}…" />
