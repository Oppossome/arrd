import { describe, expect, it } from 'vitest';
import { discoverRows, parseGenre, parseSort, posterCardProps } from './discover';
import { browseHref } from './links';

describe('discoverRows', () => {
	it('lists each kind’s curated rows in page order', () => {
		expect(discoverRows('movies').map((r) => r.list)).toEqual(['trending', 'popular', 'now_playing', 'upcoming', 'top_rated']);
		expect(discoverRows('series').map((r) => r.list)).toEqual(['trending', 'popular', 'airing_today', 'on_the_air', 'top_rated']);
	});
});

describe('parseSort', () => {
	it('accepts known orders and falls back to popular', () => {
		expect(parseSort('newest')).toBe('newest');
		expect(parseSort('top_rated')).toBe('top_rated');
		expect(parseSort(null)).toBe('popular');
		expect(parseSort('random')).toBe('popular');
	});
});

describe('parseGenre', () => {
	it('accepts positive integer ids only', () => {
		expect(parseGenre('101')).toBe(101);
		expect(parseGenre(null)).toBeUndefined();
		expect(parseGenre('0')).toBeUndefined();
		expect(parseGenre('-3')).toBeUndefined();
		expect(parseGenre('1.5')).toBeUndefined();
		expect(parseGenre('drama')).toBeUndefined();
		expect(parseGenre('99999999999999999999')).toBeUndefined();
	});
});

describe('posterCardProps', () => {
	it('links to the discover page and marks library titles', () => {
		const item = { tmdb_id: 900001, title: 'The Glass Orchard', original_title: '', year: 1999, overview: '', poster_path: '/abc.jpg' };
		expect(posterCardProps('movies', item)).toEqual({
			href: '/movies/900001',
			title: 'The Glass Orchard',
			year: 1999,
			posterSrc: '/api/v1/posters/w342/abc.jpg',
			subtitle: undefined
		});
		expect(posterCardProps('series', { ...item, library_id: 4 })).toMatchObject({ href: '/shows/900001', subtitle: 'In library' });
	});
});

describe('browseHref', () => {
	it('adds only the options given', () => {
		expect(browseHref('movies')).toBe('/movies/browse');
		expect(browseHref('series', { genre: 201 })).toBe('/shows/browse?genre=201');
		expect(browseHref('movies', { genre: 101, sort: 'newest' })).toBe('/movies/browse?genre=101&sort=newest');
	});
});
