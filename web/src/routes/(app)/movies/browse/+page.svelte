<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { BrowseGrid } from '#lib/components/organisms/index.js';
	import { ListLayout } from '#lib/components/templates/index.js';
	import { DEFAULT_SORT, parseGenre, parseSort } from '#lib/discover.js';
	import { browseHref } from '#lib/links.js';
	import type { DiscoverSort } from '#lib/types.js';

	const genre = $derived(parseGenre(page.url.searchParams.get('genre')));
	const sort = $derived(parseSort(page.url.searchParams.get('sort')));

	function change(next: { genre?: number; sort: DiscoverSort }) {
		const href = browseHref('movies', { genre: next.genre, sort: next.sort === DEFAULT_SORT ? undefined : next.sort });
		// Keep scroll and focus on the select the user just changed.
		void goto(href, { replace: true, reset: false });
	}
</script>

<ListLayout title="Browse movies">
	<BrowseGrid kind="movies" {genre} {sort} onchange={change} />
</ListLayout>
