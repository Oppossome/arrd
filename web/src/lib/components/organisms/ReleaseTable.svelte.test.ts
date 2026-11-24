import { fireEvent, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { mockFetch, renderWithClient } from '../../../test/helpers';
import type { ReleaseResult } from '#lib/api/types.js';
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
					query: 'The Glass Orchard 1999',
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
				Response.json({
					query: 'Harbor Watch S02',
					releases: [release({ title: 'Harbor.Watch.S02.1080p.WEB-DL-QUILL', season: 2, season_pack: true })],
					indexer_failures: []
				}),
			'POST /api/v1/discover/series/800001/grab': () => Response.json({ id: 1 }, { status: 201 })
		});
		renderWithClient(ReleaseTable, { source: { kind: 'discoverSeason' as const, tmdbId: 800001, season: 2 }, auto: true });

		await fireEvent.click(await screen.findByRole('button', { name: 'Grab' }));
		await waitFor(() =>
			expect(fetch).toHaveBeenCalledWith('/api/v1/discover/series/800001/grab', expect.objectContaining({ method: 'POST' }))
		);
	});

	it('puts its heading, sort and search-again controls in one header', async () => {
		let searches = 0;
		mockFetch({
			'GET /api/v1/discover/series/800001/seasons/1/episodes/2/releases': () => {
				searches++;
				return Response.json({
					query: 'Harbor Watch S01E02',
					releases: [release({ title: 'Harbor.Watch.S01E02.1080p.WEB-DL-QUILL' })],
					indexer_failures: []
				});
			}
		});
		renderWithClient(ReleaseTable, {
			source: { kind: 'discoverEpisode' as const, tmdbId: 800001, season: 1, episode: 2 },
			heading: 'Sources · Harbor Watch · S01E02',
			auto: true
		});

		const header = await screen.findByRole('heading', { name: 'Sources · Harbor Watch · S01E02' });
		const toolbar = header.parentElement!;
		const again = await within(toolbar).findByRole('button', { name: 'Search again' });
		expect(within(toolbar).getByRole('combobox')).toBeInTheDocument();

		await fireEvent.click(again);
		await waitFor(() => expect(searches).toBe(2));
	});

	it('searches the indexers for text the user types, and back for its own', async () => {
		const asked: (string | null)[] = [];
		mockFetch({
			'GET /api/v1/discover/series/800001/releases': (url) => {
				const q = url.searchParams.get('q');
				asked.push(q);
				const title = q ? 'Harbor.Watch.Complete.Series.720p.HDTV-PIKA' : 'Harbor.Watch.S01-S02.1080p.WEB-DL-QUILL';
				return Response.json({ query: q ?? 'Harbor Watch', releases: [release({ title })], indexer_failures: [] });
			}
		});
		renderWithClient(ReleaseTable, { source: { kind: 'discoverSeries' as const, tmdbId: 800001 }, auto: true });

		const box = await screen.findByRole('searchbox', { name: 'Search indexers for' });
		await waitFor(() => expect(box).toHaveValue('Harbor Watch'));
		expect(screen.queryByRole('button', { name: 'Reset' })).not.toBeInTheDocument();

		await fireEvent.input(box, { target: { value: 'Harbor Watch complete' } });
		await fireEvent.submit(screen.getByRole('search'));
		expect(await screen.findByText('Harbor.Watch.Complete.Series.720p.HDTV-PIKA')).toBeInTheDocument();
		expect(asked).toEqual([null, 'Harbor Watch complete']);

		await fireEvent.click(screen.getByRole('button', { name: 'Reset' }));
		expect(await screen.findByText('Harbor.Watch.S01-S02.1080p.WEB-DL-QUILL')).toBeInTheDocument();
		expect(box).toHaveValue('Harbor Watch');
		// arrd's own results were cached: no new search.
		expect(asked).toEqual([null, 'Harbor Watch complete']);
	});

	it('narrows the results with filter chips, and clears them', async () => {
		mockFetch({
			'GET /api/v1/movies/5/releases': () =>
				Response.json({
					query: 'The Glass Orchard 1999',
					releases: [
						release({ title: 'The.Glass.Orchard.1999.1080p.BluRay-QUILL', indexer: 'alpha' }),
						release({ title: 'The.Glass.Orchard.1999.720p.WEB-PIKA', indexer: 'beta', quality: 'WEBDL-720p', score: 700 }),
						release({ title: 'The.Glass.Orchard.1999.HDCAM-BAD', indexer: 'beta', accepted: false, rejections: ['CAM/telesync source', 'unknown resolution'] })
					],
					indexer_failures: []
				})
		});
		renderWithClient(ReleaseTable, { source: { kind: 'movie' as const, movieId: 5 }, auto: true });

		const list = await screen.findByRole('list', { name: 'Releases' });
		expect(within(list).getAllByRole('listitem')).toHaveLength(3);
		// Every reason on one line.
		expect(screen.getByText('CAM/telesync source · unknown resolution')).toBeInTheDocument();

		await fireEvent.click(screen.getByRole('button', { name: /^Accepted only/ }));
		expect(within(list).getAllByRole('listitem')).toHaveLength(2);
		expect(screen.getByText('Showing 2 of 3')).toBeInTheDocument();

		await fireEvent.click(screen.getByRole('button', { name: /^alpha/ }));
		expect(within(list).getAllByRole('listitem')).toHaveLength(1);
		expect(screen.getByRole('button', { name: /^alpha/ })).toHaveAttribute('aria-pressed', 'true');

		// Nothing left: offer to clear the filters.
		await fireEvent.click(screen.getByRole('button', { name: /^WEBDL-720p/ }));
		expect(screen.getByText('No releases match these filters.')).toBeInTheDocument();
		await fireEvent.click(screen.getByRole('button', { name: 'Clear filters' }));
		expect(within(await screen.findByRole('list', { name: 'Releases' })).getAllByRole('listitem')).toHaveLength(3);
	});
});
