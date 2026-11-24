import { describe, expect, it } from 'vitest';
import { discoverRows, parseSort, posterCardProps } from './discover';
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
		expect(browseHref('movies')).toBe('/movies');
		expect(browseHref('series', { genres: [201] })).toBe('/shows?genre=201');
		expect(browseHref('movies', { genres: [101, 102], language: 'qq', sort: 'newest' })).toBe(
			'/movies?genre=101&genre=102&lang=qq&sort=newest'
		);
		expect(browseHref('series', { title: 'harbor watch', genres: [] })).toBe('/shows?q=harbor+watch');
	});
});
