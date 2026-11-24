import { describe, expect, it } from 'vitest';
import {
	ANIME,
	NO_FACETS,
	applySuggestion,
	describeSuggestion,
	hasFacets,
	hasFilter,
	parseFacets,
	removeField,
	suggest,
	tokens,
	type Facets
} from './facets';
import { browseHref } from '../nav/links';

const genres = [
	{ id: 16, name: 'Animation' },
	{ id: 101, name: 'Quiet Drama' },
	{ id: 102, name: 'Loud Comedy' }
];
const languages = [
	{ code: 'ja', name: 'Japanese' },
	{ code: 'qq', name: 'Quenish' },
	{ code: 'zz', name: 'Zedish' }
];

describe('parseFacets', () => {
	it('reads title, genres and language', () => {
		expect(parseFacets(new URLSearchParams('q=+glass+orchard+&genre=101&genre=102&lang=qq'))).toEqual({
			title: 'glass orchard',
			genres: [101, 102],
			language: 'qq'
		});
	});
	it('ignores malformed and repeated values', () => {
		expect(parseFacets(new URLSearchParams('q=+&genre=0&genre=-3&genre=1.5&genre=drama&genre=101&genre=101&genre=99999999999999999999&lang=QQ'))).toEqual({
			title: undefined,
			genres: [101],
			language: undefined
		});
	});
	it('round-trips through browseHref', () => {
		const f: Facets = { title: 'paper lanterns', genres: [101, 16], language: 'ja' };
		const href = browseHref('movies', f);
		expect(parseFacets(new URL(href, 'http://x').searchParams)).toEqual(f);
	});
});

describe('hasFacets / hasFilter', () => {
	it('tells a title search from a filter', () => {
		expect(hasFacets(NO_FACETS)).toBe(false);
		expect(hasFacets({ genres: [], title: 'x' })).toBe(true);
		expect(hasFilter({ genres: [], title: 'x' } as Facets)).toBe(false);
		expect(hasFilter({ genres: [101] })).toBe(true);
		expect(hasFilter({ genres: [], language: 'qq' })).toBe(true);
	});
});

describe('tokens', () => {
	it('shows one token per field, genres joined with "or"', () => {
		expect(tokens({ title: 'glass', genres: [101, 102], language: 'qq' }, genres, languages)).toEqual([
			{ field: 'title', label: 'Title', value: 'glass' },
			{ field: 'genre', label: 'Genre', value: 'Quiet Drama or Loud Comedy' },
			{ field: 'language', label: 'Language', value: 'Quenish' }
		]);
	});
	it('falls back to ids and codes before names load', () => {
		expect(tokens({ genres: [101], language: 'qq' })).toEqual([
			{ field: 'genre', label: 'Genre', value: '#101' },
			{ field: 'language', label: 'Language', value: 'qq' }
		]);
	});
});

describe('removeField', () => {
	const f: Facets = { title: 'glass', genres: [101], language: 'qq' };
	it('clears just that field', () => {
		expect(removeField(f, 'title')).toEqual({ title: undefined, genres: [101], language: 'qq' });
		expect(removeField(f, 'genre')).toEqual({ title: 'glass', genres: [], language: 'qq' });
		expect(removeField(f, 'language')).toEqual({ title: 'glass', genres: [101], language: undefined });
	});
});

describe('suggest', () => {
	it('offers nothing for blank text', () => {
		expect(suggest('  ', NO_FACETS, genres, languages)).toEqual([]);
	});
	it('offers a title search first, then matching anime, genres and languages', () => {
		expect(suggest('anim', NO_FACETS, genres, languages)).toEqual([
			{ kind: 'title', title: 'anim' },
			{ kind: 'anime' },
			{ kind: 'genre', genre: genres[0] }
		]);
		expect(suggest(' quen ', NO_FACETS, genres, languages)).toEqual([
			{ kind: 'title', title: 'quen' },
			{ kind: 'language', language: languages[1] }
		]);
	});
	it('puts a filter named exactly by the text before the title search', () => {
		expect(suggest('Anime', NO_FACETS, genres, languages)[0]).toEqual({ kind: 'anime' });
		expect(suggest('animation', NO_FACETS, genres, languages).slice(0, 2)).toEqual([
			{ kind: 'genre', genre: genres[0] },
			{ kind: 'title', title: 'animation' }
		]);
		expect(suggest('quenish', NO_FACETS, genres, languages)[0]).toEqual({ kind: 'language', language: languages[1] });
	});
	it('matches a language by its code', () => {
		expect(suggest('ZZ', NO_FACETS, genres, languages)).toContainEqual({ kind: 'language', language: languages[2] });
	});
	it('offers anime for "japanese" too', () => {
		expect(suggest('japanese', NO_FACETS, genres, languages)).toEqual([
			{ kind: 'language', language: languages[0] },
			{ kind: 'title', title: 'japanese' },
			{ kind: 'anime' }
		]);
	});
	it('leaves out what is already applied', () => {
		const anime: Facets = { ...ANIME, genres: [...ANIME.genres] };
		expect(suggest('anim', anime, genres, languages)).toEqual([{ kind: 'title', title: 'anim' }]);
		expect(suggest('quen', { genres: [], language: 'qq' }, genres, languages)).toEqual([{ kind: 'title', title: 'quen' }]);
	});
	it('caps genres and languages at five each', () => {
		const many = Array.from({ length: 8 }, (_, i) => ({ id: 300 + i, name: `Harbor Tale ${i}` }));
		expect(suggest('harbor', NO_FACETS, many, []).filter((s) => s.kind === 'genre')).toHaveLength(5);
	});
});

describe('applySuggestion', () => {
	const f: Facets = { genres: [101], language: 'qq' };
	it('replaces the title and language, and adds genres', () => {
		expect(applySuggestion(f, { kind: 'title', title: 'glass' })).toEqual({ ...f, title: 'glass' });
		expect(applySuggestion(f, { kind: 'language', language: languages[2] })).toEqual({ genres: [101], language: 'zz' });
		expect(applySuggestion(f, { kind: 'genre', genre: genres[2] })).toEqual({ genres: [101, 102], language: 'qq' });
	});
	it('sets anime as Animation in Japanese, replacing other genres', () => {
		expect(applySuggestion({ ...f, title: 'glass' }, { kind: 'anime' })).toEqual({ title: 'glass', genres: [16], language: 'ja' });
	});
});

describe('describeSuggestion', () => {
	it('names the field and value', () => {
		expect(describeSuggestion({ kind: 'title', title: 'glass' })).toEqual({ label: 'Search title for', value: 'glass' });
		expect(describeSuggestion({ kind: 'anime' })).toEqual({ label: 'Anime', value: 'Japanese animation' });
		expect(describeSuggestion({ kind: 'genre', genre: genres[1] })).toEqual({ label: 'Genre', value: 'Quiet Drama' });
		expect(describeSuggestion({ kind: 'language', language: languages[1] })).toEqual({ label: 'Language', value: 'Quenish' });
	});
});
