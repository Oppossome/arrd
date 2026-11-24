import { describe, expect, it } from 'vitest';
import { releaseCoverage, sortReleases } from './releases';
import type { ReleaseResult } from '../api/types';

const rel = (title: string, o: Partial<ReleaseResult>): ReleaseResult => ({
	title,
	indexer_id: 1,
	indexer: 'idx',
	size: 0,
	seeders: 0,
	peers: 0,
	publish_date: '2025-01-01T00:00:00Z',
	quality: 'WEBDL-1080p',
	accepted: true,
	score: 0,
	...o
});

describe('sortReleases', () => {
	const list = [
		rel('a', { accepted: true, score: 100, seeders: 5, size: 3 }),
		rel('b', { accepted: false, score: 900, seeders: 99, size: 9 }),
		rel('c', { accepted: true, score: 200, seeders: 1, size: 1 })
	];
	it('keeps the server ranking for "rank"', () => {
		expect(sortReleases(list, 'rank').map((r) => r.title)).toEqual(['a', 'b', 'c']);
	});
	it('sorts accepted releases first, then by the key', () => {
		expect(sortReleases(list, 'quality').map((r) => r.title)).toEqual(['c', 'a', 'b']);
		expect(sortReleases(list, 'seeders').map((r) => r.title)).toEqual(['a', 'c', 'b']);
	});
	it('does not mutate its input', () => {
		sortReleases(list, 'size');
		expect(list.map((r) => r.title)).toEqual(['a', 'b', 'c']);
	});
});

describe('releaseCoverage', () => {
	it('describes TV coverage', () => {
		expect(releaseCoverage(rel('x', {}))).toBe('');
		expect(releaseCoverage(rel('x', { season: 1, season_pack: true }))).toBe('S01 pack');
		expect(releaseCoverage(rel('x', { season: 2, episodes: [3] }))).toBe('S02E03');
		expect(releaseCoverage(rel('x', { season: 2, episodes: [3, 4, 5] }))).toBe('S02E03-E05');
		expect(releaseCoverage(rel('x', { season: 1, last_season: 3, season_pack: true }))).toBe('S01-S03 pack');
		expect(releaseCoverage(rel('x', { season: 0, complete_series: true, season_pack: true }))).toBe('Complete series');
	});
});
