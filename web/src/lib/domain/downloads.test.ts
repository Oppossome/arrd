import { describe, expect, it } from 'vitest';
import { activeDownloads, itemDownloads } from './downloads';
import type { Download, Progress } from '../api/types';

let nextId = 1;
function dl(over: Partial<Download>): Download {
	return {
		id: nextId++,
		info_hash: 'a'.repeat(40),
		release_title: 'Some.Release.1080p-GRP',
		quality: 'WEBDL-1080p',
		size: 1000,
		status: 'downloading',
		paused: false,
		added_at: '2026-11-20T10:00:00Z',
		...over
	};
}

function progress(done: number, total: number): Progress {
	return {
		info_hash: 'a'.repeat(40),
		name: 'x',
		have_info: true,
		bytes_completed: done,
		bytes_total: total,
		active_peers: 1,
		total_peers: 1,
		seeders: 1,
		complete: done === total,
		download_rate: 0,
		upload_rate: 0,
		eta_seconds: 0,
		countries: []
	};
}

describe('itemDownloads', () => {
	it('sums running downloads per item', () => {
		const { series } = itemDownloads([
			dl({ series_id: 7, progress: progress(250, 1000) }),
			dl({ series_id: 7, status: 'completed', size: 1000 }),
			dl({ series_id: 7, status: 'imported', progress: progress(500, 500) })
		]);
		expect(series.get(7)).toEqual({ kind: 'active', percent: 62.5, paused: false, importing: false });
	});

	it('is paused only when every running download is', () => {
		const { movies } = itemDownloads([
			dl({ movie_id: 1, paused: true, progress: progress(1, 10) }),
			dl({ movie_id: 2, paused: true }),
			dl({ movie_id: 2, paused: false })
		]);
		expect(movies.get(1)).toMatchObject({ kind: 'active', paused: true, percent: 10 });
		expect(movies.get(2)).toMatchObject({ paused: false });
	});

	it('marks an item failed only when its newest download failed', () => {
		const { movies } = itemDownloads([
			dl({ movie_id: 1, status: 'failed', added_at: '2026-11-21T00:00:00Z' }),
			dl({ movie_id: 1, status: 'imported', added_at: '2026-11-20T00:00:00Z' }),
			dl({ movie_id: 2, status: 'failed', added_at: '2026-11-19T00:00:00Z' }),
			dl({ movie_id: 2, status: 'imported', added_at: '2026-11-20T00:00:00Z' }),
			dl({ movie_id: 3, status: 'imported' })
		]);
		expect(movies.get(1)).toEqual({ kind: 'failed' });
		expect(movies.has(2)).toBe(false);
		expect(movies.has(3)).toBe(false);
	});

	it('prefers a running download over an older failure', () => {
		const { movies } = itemDownloads([
			dl({ movie_id: 1, status: 'failed', added_at: '2026-11-22T00:00:00Z' }),
			dl({ movie_id: 1, status: 'completed', added_at: '2026-11-21T00:00:00Z' })
		]);
		expect(movies.get(1)).toEqual({ kind: 'active', percent: 100, paused: false, importing: true });
	});
});

describe('activeDownloads', () => {
	it('keeps running downloads, newest first', () => {
		const old = dl({ movie_id: 1, added_at: '2026-11-01T00:00:00Z' });
		const recent = dl({ series_id: 2, status: 'completed', added_at: '2026-11-02T00:00:00Z' });
		const done = dl({ movie_id: 3, status: 'imported' });
		const failed = dl({ movie_id: 4, status: 'failed' });
		expect(activeDownloads([old, done, recent, failed])).toEqual([recent, old]);
	});
});
