<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { FacetSearchBar, type FacetItem } from '#lib/components/molecules/index.js';
	import {
		applySuggestion,
		describeSuggestion,
		removeField,
		suggest,
		tokens,
		type FacetField,
		type Facets,
		type Suggestion
	} from '#lib/domain/facets.js';
	import type { MediaKind } from '#lib/nav/links.js';
	import { queries } from '#lib/api/query.js';

	interface Props {
		kind: MediaKind;
		facets: Facets;
		/** The user added or removed a filter. */
		onchange: (next: Facets) => void;
	}
	let { kind, facets, onchange }: Props = $props();

	// Suggestions only need these once loaded; until then the bar still
	// offers a title search, and tokens show ids and codes.
	const genres = createQuery(() => queries.genres(kind));
	const languages = createQuery(() => queries.languages());

	let text = $state('');
	const noun = $derived(kind === 'movies' ? 'movies' : 'shows');

	const suggestions = $derived(suggest(text, facets, genres.data ?? [], languages.data ?? []));
	const suggestionItems = $derived(suggestions.map((s): FacetItem => ({ key: suggestionKey(s), ...describeSuggestion(s) })));
	const tokenItems = $derived(
		tokens(facets, genres.data, languages.data).map((t): FacetItem => ({ key: t.field, label: t.label, value: t.value }))
	);

	function suggestionKey(s: Suggestion): string {
		switch (s.kind) {
			case 'genre':
				return `genre-${s.genre.id}`;
			case 'language':
				return `language-${s.language.code}`;
			default:
				return s.kind;
		}
	}

	function pick(index: number) {
		// Read the suggestion first: clearing the text empties the list.
		const next = applySuggestion(facets, suggestions[index]);
		text = '';
		onchange(next);
	}
</script>

<FacetSearchBar
	tokens={tokenItems}
	suggestions={suggestionItems}
	bind:text
	label="Search {noun}"
	placeholder="Search {noun}, or filter by genre or language…"
	onpick={pick}
	onremove={(field) => onchange(removeField(facets, field as FacetField))}
/>
