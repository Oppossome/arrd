import { screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { mockFetch, renderWithClient } from '../../../test/helpers';
import { lookupMovie } from '../../../test/discover';
import DiscoverListRow from './DiscoverListRow.svelte';

afterEach(() => vi.unstubAllGlobals());

describe('DiscoverListRow', () => {
	it('shows a curated list as a row', async () => {
		mockFetch({
			'GET /api/v1/discover/lists/movies/now_playing': () =>
				Response.json([lookupMovie(900002, 'Paper Lanterns', 3), lookupMovie(900003, 'Salt Roads')])
		});
		renderWithClient(DiscoverListRow, { kind: 'movies' as const, list: 'now_playing' as const, title: 'In theaters' });

		// A skeleton row shows first; the loaded row replaces it.
		await screen.findByRole('link', { name: /Salt Roads/ });
		const row = screen.getByRole('region', { name: 'In theaters' });
		expect(within(row).getByRole('link', { name: /Salt Roads/ })).toHaveAttribute('href', '/movies/900003');
		expect(within(row).getByText(/In library/)).toBeInTheDocument();
	});

	it('shows the error under the row title', async () => {
		mockFetch({
			'GET /api/v1/discover/lists/series/airing_today': () => Response.json({ error: 'TMDB is down' }, { status: 502 })
		});
		renderWithClient(DiscoverListRow, { kind: 'series' as const, list: 'airing_today' as const, title: 'Airing today' });

		await screen.findByText(/TMDB is down/);
		expect(within(screen.getByRole('region', { name: 'Airing today' })).getByText(/TMDB is down/)).toBeInTheDocument();
	});

	it('hides an empty list', async () => {
		const fetch = mockFetch({ 'GET /api/v1/discover/lists/series/on_the_air': () => Response.json([]) });
		renderWithClient(DiscoverListRow, { kind: 'series' as const, list: 'on_the_air' as const, title: 'On the air' });

		await waitFor(() => expect(fetch).toHaveBeenCalled());
		await waitFor(() => expect(screen.queryByRole('region', { name: 'On the air' })).not.toBeInTheDocument());
	});
});
