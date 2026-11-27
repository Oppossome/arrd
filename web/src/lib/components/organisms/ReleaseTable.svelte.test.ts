import { fireEvent, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { mockFetch, renderWithClient } from '../../../test/helpers';
import type { ReleaseResult, ReleaseTarget, SearchResult } from '#lib/api/types.js';
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

/**
 * Serves GET /api/v1/releases with result (or result(params)) and accepts
 * grabs; asked collects each search's query parameters.
 */
function mockReleases(result: SearchResult | ((params: URLSearchParams) => SearchResult)) {
	const asked: URLSearchParams[] = [];
	const fetch = mockFetch({
		'GET /api/v1/releases': (url) => {
			asked.push(url.searchParams);
			return Response.json(typeof result === 'function' ? result(url.searchParams) : result);
		},
		'POST /api/v1/grabs': () => Response.json({ id: 1 }, { status: 201 })
	});
	const grabBody = () =>
		JSON.parse((fetch.mock.calls.at(-1)![1] as RequestInit).body as string) as { target: ReleaseTarget; release: ReleaseResult };
	return { fetch, asked, grabBody };
}

describe('ReleaseTable', () => {
	it('searches on demand, explains rejections and grabs', async () => {
		const { fetch, asked, grabBody } = mockReleases({
			query: 'The Glass Orchard 1999',
			releases: [release({}), release({ title: 'The.Glass.Orchard.1999.HDCAM-BAD', accepted: false, rejections: ['CAM/telesync source'] })],
			indexer_failures: [{ indexer_id: 2, indexer: 'broken', error: 'connection refused' }]
		});
		const target = { kind: 'movie' as const, tmdb_id: 900001 };
		renderWithClient(ReleaseTable, { target });

		// Nothing is fetched until the user asks.
		expect(fetch).not.toHaveBeenCalled();
		await fireEvent.click(screen.getByRole('button', { name: 'Search' }));

		expect(await screen.findByText('CAM/telesync source')).toBeInTheDocument();
		expect(Object.fromEntries(asked[0])).toEqual({ kind: 'movie', tmdb_id: '900001' });
		expect(screen.getByText('broken: connection refused')).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Grab anyway' })).toBeInTheDocument();

		await fireEvent.click(screen.getByRole('button', { name: 'Grab' }));
		await waitFor(() => expect(fetch).toHaveBeenCalledWith('/api/v1/grabs', expect.objectContaining({ method: 'POST' })));
		expect(grabBody().target).toEqual(target);
		expect(grabBody().release.title).toBe('The.Glass.Orchard.1999.1080p.BluRay.x264-QUILL');
	});

	it.each([
		{
			target: { kind: 'season' as const, tmdb_id: 800001, season: 2 },
			params: { kind: 'season', tmdb_id: '800001', season: '2' },
			result: release({ title: 'Harbor.Watch.S02.1080p.WEB-DL-QUILL', season: 2, season_pack: true }),
			coverage: 'S02 pack'
		},
		{
			target: { kind: 'collection' as const, tmdb_id: 900011 },
			params: { kind: 'collection', tmdb_id: '900011' },
			result: release({ title: 'Skyfarer.Trilogy.2011-2015.1080p.BluRay.x264-QUILL', movie_pack: true }),
			coverage: 'Movie pack'
		}
	])('searches a $target.kind at once and grabs it for that target', async ({ target, params, result, coverage }) => {
		const { fetch, asked, grabBody } = mockReleases({ query: 'x', releases: [result], indexer_failures: [] });
		renderWithClient(ReleaseTable, { target, auto: true });

		expect(await screen.findByText(coverage)).toBeInTheDocument();
		expect(Object.fromEntries(asked[0])).toEqual(params);
		await fireEvent.click(screen.getByRole('button', { name: 'Grab' }));
		await waitFor(() => expect(fetch).toHaveBeenCalledWith('/api/v1/grabs', expect.objectContaining({ method: 'POST' })));
		expect(grabBody().target).toEqual(target);
	});

	it('puts its heading, search and sort controls in one header', async () => {
		const { asked } = mockReleases({
			query: 'Harbor Watch S01E02',
			releases: [release({ title: 'Harbor.Watch.S01E02.1080p.WEB-DL-QUILL' })],
			indexer_failures: []
		});
		renderWithClient(ReleaseTable, {
			target: { kind: 'episode' as const, tmdb_id: 800001, season: 1, episode: 2 },
			heading: 'Sources · Harbor Watch · S01E02',
			auto: true
		});

		const header = await screen.findByRole('heading', { name: 'Sources · Harbor Watch · S01E02' });
		const toolbar = header.parentElement!;
		expect(await within(toolbar).findByRole('button', { name: 'Sort: Best match' })).toBeInTheDocument();
		const again = within(toolbar).getByRole('button', { name: 'Search' });

		await fireEvent.click(again);
		await waitFor(() => expect(asked).toHaveLength(2));
		expect(Object.fromEntries(asked[1])).toEqual({ kind: 'episode', tmdb_id: '800001', season: '1', episode: '2' });
	});

	it('searches the indexers for text the user types, and back for its own', async () => {
		const { asked } = mockReleases((params) => {
			const q = params.get('q');
			const title = q ? 'Harbor.Watch.Complete.Series.720p.HDTV-PIKA' : 'Harbor.Watch.S01-S02.1080p.WEB-DL-QUILL';
			return { query: q ?? 'Harbor Watch', releases: [release({ title })], indexer_failures: [] };
		});
		renderWithClient(ReleaseTable, { target: { kind: 'series' as const, tmdb_id: 800001 }, auto: true });

		const box = await screen.findByRole('searchbox', { name: 'Search indexers for' });
		await waitFor(() => expect(box).toHaveValue('Harbor Watch'));
		expect(screen.queryByRole('button', { name: 'Reset' })).not.toBeInTheDocument();

		await fireEvent.input(box, { target: { value: 'Harbor Watch complete' } });
		await fireEvent.submit(screen.getByRole('search'));
		expect(await screen.findByText('Harbor.Watch.Complete.Series.720p.HDTV-PIKA')).toBeInTheDocument();
		expect(asked.map((p) => p.get('q'))).toEqual([null, 'Harbor Watch complete']);

		await fireEvent.click(screen.getByRole('button', { name: 'Reset' }));
		expect(await screen.findByText('Harbor.Watch.S01-S02.1080p.WEB-DL-QUILL')).toBeInTheDocument();
		expect(box).toHaveValue('Harbor Watch');
		// arrd's own results were cached: no new search.
		expect(asked).toHaveLength(2);
	});

	it('narrows the results with filter menus, and clears them', async () => {
		mockReleases({
			query: 'The Glass Orchard 1999',
			releases: [
				release({ title: 'The.Glass.Orchard.1999.1080p.BluRay-QUILL', indexer: 'alpha' }),
				release({ title: 'The.Glass.Orchard.1999.720p.WEB-PIKA', indexer: 'beta', quality: 'WEBDL-720p', score: 700 }),
				release({ title: 'The.Glass.Orchard.1999.HDCAM-BAD', indexer: 'beta', accepted: false, rejections: ['CAM/telesync source', 'unknown resolution'] })
			],
			indexer_failures: []
		});
		renderWithClient(ReleaseTable, { target: { kind: 'movie' as const, tmdb_id: 900001 }, auto: true });

		const list = await screen.findByRole('list', { name: 'Releases' });
		expect(within(list).getAllByRole('listitem')).toHaveLength(3);
		// Every reason on one line.
		expect(screen.getByText('CAM/telesync source · unknown resolution')).toBeInTheDocument();

		await fireEvent.click(screen.getByRole('button', { name: /^Accepted/ }));
		expect(within(list).getAllByRole('listitem')).toHaveLength(2);
		expect(screen.getByText('Showing 2 of 3 results')).toBeInTheDocument();

		await fireEvent.click(screen.getByRole('button', { name: /^Indexer/ }));
		await fireEvent.click(screen.getByRole('checkbox', { name: /^alpha/ }));
		expect(within(list).getAllByRole('listitem')).toHaveLength(1);
		expect(screen.getByRole('checkbox', { name: /^alpha/ })).toBeChecked();
		expect(screen.getByRole('button', { name: /^Indexer/ })).toHaveTextContent('(1)');

		// Nothing left: offer to clear the filters.
		await fireEvent.click(screen.getByRole('button', { name: /^Quality/ }));
		await fireEvent.click(screen.getByRole('checkbox', { name: /^WEBDL-720p/ }));
		expect(screen.getByText('No releases match these filters.')).toBeInTheDocument();
		await fireEvent.click(screen.getByRole('button', { name: 'Clear filters' }));
		expect(within(await screen.findByRole('list', { name: 'Releases' })).getAllByRole('listitem')).toHaveLength(3);
	});
});
