import { fireEvent, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { mockFetch, renderWithClient } from '../../../test/helpers';
import type { LookupSeries } from '#lib/types.js';
import DiscoverHeader from './DiscoverHeader.svelte';
import DiscoverHeaderHarness from '../../../test/DiscoverHeaderHarness.svelte';

afterEach(() => vi.unstubAllGlobals());

const show: LookupSeries = {
	tmdb_id: 800001,
	title: 'Harbor Watch',
	original_title: 'Harbor Watch',
	year: 2019,
	overview: 'A fictional coastal drama.',
	library_id: 4,
	seasons: [
		{ season_number: 0, episode_count: 1 },
		{ season_number: 1, episode_count: 3 },
		{ season_number: 2, episode_count: 2 },
		{ season_number: 3, episode_count: 0 } // announced, no episodes yet
	]
};

describe('DiscoverHeader', () => {
	it('starts at the latest season with episodes and links to the library copy', async () => {
		mockFetch({ 'GET /api/v1/discover/series/800001': () => Response.json(show) });
		renderWithClient(DiscoverHeaderHarness, { tmdbId: 800001 });
		const season = () => screen.getByTestId('season').textContent;

		const select = await screen.findByLabelText<HTMLSelectElement>('Season');
		expect([...select.options].map((o) => o.text)).toEqual([
			'Season 2 (2 episodes)',
			'Season 1 (3 episodes)',
			'Specials (1 episodes)'
		]);
		expect(season()).toBe('2');
		expect(screen.getByRole('link', { name: 'Open in library' })).toHaveAttribute('href', '/library/shows/4');

		await fireEvent.change(select, { target: { value: select.options[1].value } });
		expect(season()).toBe('1');
	});
});

describe('DiscoverHeader for a movie not in the library', () => {
	it('adds it to the library as monitored', async () => {
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
		renderWithClient(DiscoverHeader, { kind: 'movies' as const, tmdbId: 900001 });

		await fireEvent.click(await screen.findByRole('button', { name: 'Add to library (monitored)' }));
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
