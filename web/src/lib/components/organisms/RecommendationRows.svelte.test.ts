import { screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { mockFetch, renderWithClient } from '../../../test/helpers';
import { lookupMovie } from '../../../test/discover';
import RecommendationRows from './RecommendationRows.svelte';

afterEach(() => vi.unstubAllGlobals());

describe('RecommendationRows', () => {
	it('shows a scrolling row per library title', async () => {
		mockFetch({
			'GET /api/v1/discover/movies/recommendations': () =>
				Response.json([
					{
						because: { tmdb_id: 900003, title: 'Salt Roads' },
						items: [lookupMovie(900010, 'Copper Tide'), lookupMovie(900011, 'Moth Season')]
					},
					{ because: { tmdb_id: 900001, title: 'The Glass Orchard' }, items: [lookupMovie(900012, 'Quiet Engines')] }
				])
		});
		renderWithClient(RecommendationRows, { kind: 'movies' as const });

		const first = await screen.findByRole('region', { name: 'Because you added Salt Roads' });
		expect(within(first).getAllByRole('link').map((a) => a.getAttribute('href'))).toEqual(['/movies/900010', '/movies/900011']);
		const second = screen.getByRole('region', { name: 'Because you added The Glass Orchard' });
		expect(within(second).getByRole('link', { name: /Quiet Engines/ })).toHaveAttribute('href', '/movies/900012');
	});

	it('shows an error when the rows fail to load', async () => {
		mockFetch({
			'GET /api/v1/discover/series/recommendations': () => Response.json({ error: 'TMDB is down' }, { status: 502 })
		});
		renderWithClient(RecommendationRows, { kind: 'series' as const });

		expect(await screen.findByText(/TMDB is down/)).toBeInTheDocument();
	});
});
