// The discover pages' search bar: the filters ("facets") it holds, how they
// live in the URL, and what it suggests as the user types.

import { wordMatcher } from '../utils/search';
import type { Genre, Language, TitleFilter } from '../api/types';

/** What the discover search bar narrows TMDB's catalogue to. */
export interface Facets extends TitleFilter {
	/** A title to search for; browse by filter only when absent. */
	title?: string;
}

export const NO_FACETS: Facets = { genres: [] };

/**
 * TMDB has no anime genre. Animation first made in Japanese is the usual
 * stand-in. Animation has the same id in TMDB's movie and TV genre lists.
 */
export const ANIME: TitleFilter = { genres: [16], language: 'ja' };

const LANGUAGE_CODE = /^[a-z]{2}$/;

/** The search bar's facets from the URL, ignoring malformed values. */
export function parseFacets(params: Pick<URLSearchParams, 'get' | 'getAll'>): Facets {
	const title = params.get('q')?.trim() || undefined;
	const genres = [...new Set(params.getAll('genre').filter((g) => /^\d+$/.test(g)).map(Number))].filter(
		(id) => id > 0 && Number.isSafeInteger(id)
	);
	const lang = params.get('lang');
	return { title, genres, language: lang !== null && LANGUAGE_CODE.test(lang) ? lang : undefined };
}

/** Whether any facet is set. */
export const hasFacets = (f: Facets) => f.title !== undefined || hasFilter(f);

/** Whether a genre or language is set, so there is something to browse by. */
export const hasFilter = (f: TitleFilter) => f.genres.length > 0 || f.language !== undefined;

// ---------------------------------------------------------------- tokens

export type FacetField = 'title' | 'genre' | 'language';

/** A facet as the search bar shows it: one per field, Odoo style. */
export interface Token {
	field: FacetField;
	/** The field's name, e.g. "Genre". */
	label: string;
	/** Its values, e.g. "Animation or Comedy". */
	value: string;
}

/**
 * The search bar's tokens for these facets. Genres and languages show their
 * names once loaded, and their ids or codes until then.
 */
export function tokens(f: Facets, genres: Genre[] = [], languages: Language[] = []): Token[] {
	const out: Token[] = [];
	if (f.title !== undefined) out.push({ field: 'title', label: 'Title', value: f.title });
	if (f.genres.length > 0) {
		const name = (id: number) => genres.find((g) => g.id === id)?.name ?? `#${id}`;
		out.push({ field: 'genre', label: 'Genre', value: f.genres.map(name).join(' or ') });
	}
	if (f.language !== undefined) {
		const name = languages.find((l) => l.code === f.language)?.name ?? f.language;
		out.push({ field: 'language', label: 'Language', value: name });
	}
	return out;
}

/** The facets without one field. */
export function removeField(f: Facets, field: FacetField): Facets {
	switch (field) {
		case 'title':
			return { ...f, title: undefined };
		case 'genre':
			return { ...f, genres: [] };
		case 'language':
			return { ...f, language: undefined };
	}
}

// ---------------------------------------------------------------- suggestions

export type Suggestion =
	| { kind: 'title'; title: string }
	| { kind: 'anime' }
	| { kind: 'genre'; genre: Genre }
	| { kind: 'language'; language: Language };

/** The most genres, and languages, suggested at once. */
export const MAX_SUGGESTIONS_PER_FIELD = 5;

/**
 * What the search bar offers for the text typed so far: a title search,
 * then the anime preset, genres and languages whose names match. A filter
 * whose name is exactly the text comes before the title search, so typing
 * "anime" and pressing Enter applies the preset. Nothing already applied is
 * offered again.
 */
export function suggest(text: string, f: Facets, genres: Genre[], languages: Language[]): Suggestion[] {
	const title = text.trim();
	if (title === '') return [];
	const matches = wordMatcher(title);
	const out: Exclude<Suggestion, { kind: 'title' }>[] = [];
	const animeApplied = f.genres.join() === ANIME.genres.join() && f.language === ANIME.language;
	if (!animeApplied && matches(['Anime', 'Japanese animation'])) out.push({ kind: 'anime' });
	out.push(
		...genres
			.filter((g) => !f.genres.includes(g.id) && matches([g.name]))
			.slice(0, MAX_SUGGESTIONS_PER_FIELD)
			.map((genre) => ({ kind: 'genre' as const, genre }))
	);
	out.push(
		...languages
			.filter((l) => l.code !== f.language && (l.code === title.toLowerCase() || matches([l.name])))
			.slice(0, MAX_SUGGESTIONS_PER_FIELD)
			.map((language) => ({ kind: 'language' as const, language }))
	);
	const named = (s: (typeof out)[number]) => filterName(s).toLowerCase() === title.toLowerCase();
	return [...out.filter(named), { kind: 'title', title }, ...out.filter((s) => !named(s))];
}

/** The name a filter suggestion goes by. */
function filterName(s: Exclude<Suggestion, { kind: 'title' }>): string {
	switch (s.kind) {
		case 'anime':
			return 'Anime';
		case 'genre':
			return s.genre.name;
		case 'language':
			return s.language.name;
	}
}

/**
 * The facets with a suggestion applied. A title or language replaces the
 * current one, and a genre joins the others (a title may have any of them).
 * Anime replaces both with Animation and Japanese: genres are OR'd, so
 * keeping others would let non-animated titles through.
 */
export function applySuggestion(f: Facets, s: Suggestion): Facets {
	switch (s.kind) {
		case 'title':
			return { ...f, title: s.title };
		case 'genre':
			return { ...f, genres: [...f.genres, s.genre.id] };
		case 'language':
			return { ...f, language: s.language.code };
		case 'anime':
			return { ...f, genres: [...ANIME.genres], language: ANIME.language };
	}
}

/** How a suggestion reads in the list: the field it sets, then the value. */
export function describeSuggestion(s: Suggestion): { label: string; value: string } {
	switch (s.kind) {
		case 'title':
			return { label: 'Search title for', value: s.title };
		case 'anime':
			return { label: 'Anime', value: 'Japanese animation' };
		case 'genre':
			return { label: 'Genre', value: s.genre.name };
		case 'language':
			return { label: 'Language', value: s.language.name };
	}
}
