import { describe, expect, it } from 'vitest';
import {
	emptyReleaseFilter,
	filterReleases,
	isFilterEmpty,
	rejectionSummary,
	releaseCoverage,
	releaseFacets,
	sortReleases,
	toggleValue
} from './releases';
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

describe('filterReleases', () => {
	const list = [
		rel('a', { accepted: true, quality: 'WEBDL-1080p', indexer: 'one' }),
		rel('b', { accepted: false, quality: 'HDTV-720p', indexer: 'one' }),
		rel('c', { accepted: true, quality: 'HDTV-720p', indexer: 'two' })
	];
	const titles = (f: Partial<ReturnType<typeof emptyReleaseFilter>>) =>
		filterReleases(list, { ...emptyReleaseFilter(), ...f }).map((r) => r.title);

	it('keeps everything with an empty filter', () => {
		expect(titles({})).toEqual(['a', 'b', 'c']);
		expect(isFilterEmpty(emptyReleaseFilter())).toBe(true);
	});
	it('narrows by acceptance, quality and indexer together', () => {
		expect(titles({ acceptedOnly: true })).toEqual(['a', 'c']);
		expect(titles({ qualities: ['HDTV-720p'] })).toEqual(['b', 'c']);
		expect(titles({ qualities: ['HDTV-720p'], indexers: ['one'] })).toEqual(['b']);
		expect(titles({ acceptedOnly: true, indexers: ['one', 'two'] })).toEqual(['a', 'c']);
		expect(isFilterEmpty({ ...emptyReleaseFilter(), indexers: ['one'] })).toBe(false);
	});
});

describe('releaseFacets', () => {
	it('orders qualities by best score and indexers by count', () => {
		const f = releaseFacets([
			rel('a', { quality: 'HDTV-720p', score: 300, indexer: 'beta' }),
			rel('b', { quality: 'WEBDL-1080p', score: 900, indexer: 'alpha' }),
			rel('c', { quality: 'HDTV-720p', score: 100, indexer: 'beta' })
		]);
		expect(f.qualities).toEqual([
			{ value: 'WEBDL-1080p', count: 1 },
			{ value: 'HDTV-720p', count: 2 }
		]);
		expect(f.indexers).toEqual([
			{ value: 'beta', count: 2 },
			{ value: 'alpha', count: 1 }
		]);
	});
});

describe('toggleValue', () => {
	it('adds a missing value and removes a present one', () => {
		expect(toggleValue(['a'], 'b')).toEqual(['a', 'b']);
		expect(toggleValue(['a', 'b'], 'a')).toEqual(['b']);
	});
});

describe('rejectionSummary', () => {
	it('puts every reason on one line', () => {
		expect(rejectionSummary(rel('a', { rejections: ['not a TV release', 'unknown resolution'] }))).toBe(
			'not a TV release · unknown resolution'
		);
		expect(rejectionSummary(rel('a', {}))).toBe('');
	});
});
