import { fireEvent, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { mockFetch, renderWithClient } from '../../../test/helpers';
import type { ReleaseResult } from '#lib/types.js';
import ReleaseTable from './ReleaseTable.svelte';

afterEach(() => vi.unstubAllGlobals());

const release = (o: Partial<ReleaseResult>): ReleaseResult => ({
	title: 'The.Glass.Orchard.1999.1080p.BluRay.x264-QUILL',
	indexer_id: 1,
	indexer: 'idx',
	size: 8 * 2 ** 30,
	seeders: 12,
	peers: 15,
	publish_date: '2025-01-01T00:00:00Z',
	quality: 'Bluray-1080p',
	accepted: true,
	score: 1060,
	...o
});

describe('ReleaseTable', () => {
	it('searches on demand, explains rejections and grabs', async () => {
		const fetch = mockFetch({
			'GET /api/v1/movies/5/releases': () =>
				Response.json({
					releases: [release({}), release({ title: 'The.Glass.Orchard.1999.HDCAM-BAD', accepted: false, rejections: ['CAM/telesync source'] })],
					indexer_failures: [{ indexer_id: 2, indexer: 'broken', error: 'connection refused' }]
				}),
			'POST /api/v1/movies/5/grab': () => Response.json({ id: 1 }, { status: 201 })
		});
		renderWithClient(ReleaseTable, { source: { kind: 'movie' as const, movieId: 5 } });

		// Nothing is fetched until the user asks.
		expect(fetch).not.toHaveBeenCalled();
		await fireEvent.click(screen.getByRole('button', { name: 'Search indexers' }));

		expect(await screen.findByText('CAM/telesync source')).toBeInTheDocument();
		expect(screen.getByText('broken: connection refused')).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Grab anyway' })).toBeInTheDocument();

		await fireEvent.click(screen.getByRole('button', { name: 'Grab' }));
		await waitFor(() => expect(fetch).toHaveBeenCalledWith('/api/v1/movies/5/grab', expect.objectContaining({ method: 'POST' })));
		const body = JSON.parse((fetch.mock.calls.at(-1)![1] as RequestInit).body as string) as ReleaseResult;
		expect(body.title).toBe('The.Glass.Orchard.1999.1080p.BluRay.x264-QUILL');
	});

	it('searches a discovered season at once and grabs through discover', async () => {
		const fetch = mockFetch({
			'GET /api/v1/discover/series/800001/seasons/2/releases': () =>
				Response.json({ releases: [release({ title: 'Harbor.Watch.S02.1080p.WEB-DL-QUILL', season: 2, season_pack: true })], indexer_failures: [] }),
			'POST /api/v1/discover/series/800001/grab': () => Response.json({ id: 1 }, { status: 201 })
		});
		renderWithClient(ReleaseTable, { source: { kind: 'discoverSeason' as const, tmdbId: 800001, season: 2 }, auto: true });

		await fireEvent.click(await screen.findByRole('button', { name: 'Grab' }));
		await waitFor(() =>
			expect(fetch).toHaveBeenCalledWith('/api/v1/discover/series/800001/grab', expect.objectContaining({ method: 'POST' }))
		);
	});
});
