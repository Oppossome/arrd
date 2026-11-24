import { fireEvent, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { mockFetch, renderWithClient } from '../../../test/helpers';
import DiscoverActions from './DiscoverActions.svelte';

afterEach(() => vi.unstubAllGlobals());

describe('DiscoverActions for a movie not in the library', () => {
	it('offers its sources and adds it to the library as monitored', async () => {
		let inLibrary = false;
		const fetch = mockFetch({
			'GET /api/v1/discover/movies/900001': () =>
				Response.json({
					tmdb_id: 900001,
					title: 'The Glass Orchard',
					original_title: 'The Glass Orchard',
					year: 1999,
					overview: 'A fictional orchard mystery.',
					...(inLibrary ? { library_id: 7 } : {})
				}),
			'POST /api/v1/movies': () => {
				inLibrary = true;
				return Response.json({ id: 7 }, { status: 201 });
			}
		});
		const ondownload = vi.fn();
		renderWithClient(DiscoverActions, { kind: 'movies' as const, tmdbId: 900001, ondownload });

		await fireEvent.click(await screen.findByRole('button', { name: 'Download' }));
		expect(ondownload).toHaveBeenCalledWith({
			title: 'The Glass Orchard',
			source: { kind: 'discoverMovie', tmdbId: 900001 }
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Add to library (monitored)' }));
		const post = await waitFor(() => {
			const call = fetch.mock.calls.find(([, i]) => i?.method === 'POST');
			expect(call).toBeDefined();
			return call;
		});
		expect(JSON.parse(post?.[1]?.body as string)).toEqual({ tmdb_id: 900001, monitored: true, search: true });

		// The page refreshes and links to the library copy instead.
		expect(await screen.findByRole('link', { name: 'Open in library' })).toHaveAttribute('href', '/library/movies/7');
		expect(screen.queryByRole('button', { name: 'Add to library (monitored)' })).not.toBeInTheDocument();
	});
});

describe('DiscoverActions for a series in the library', () => {
	it('links to the library copy and offers whole-series packs', async () => {
		mockFetch({
			'GET /api/v1/discover/series/800001': () =>
				Response.json({
					tmdb_id: 800001,
					title: 'Harbor Watch',
					original_title: 'Harbor Watch',
					year: 2019,
					overview: 'A fictional coastal drama.',
					library_id: 4
				})
		});
		const ondownload = vi.fn();
		renderWithClient(DiscoverActions, { kind: 'series' as const, tmdbId: 800001, ondownload });

		expect(await screen.findByRole('link', { name: 'Open in library' })).toHaveAttribute('href', '/library/shows/4');
		expect(screen.queryByRole('button', { name: 'Download' })).not.toBeInTheDocument();

		await fireEvent.click(screen.getByRole('button', { name: 'Download series' }));
		expect(ondownload).toHaveBeenCalledWith({
			title: 'Harbor Watch · Whole series',
			source: { kind: 'discoverSeries', tmdbId: 800001 }
		});
	});
});
