import { screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { mockFetch, renderWithClient } from '../../../test/helpers';
import { lookupSeries } from '../../../test/discover';
import MoreLikeThis from './MoreLikeThis.svelte';

afterEach(() => vi.unstubAllGlobals());

describe('MoreLikeThis', () => {
	it('shows TMDB’s recommendations for the title', async () => {
		mockFetch({
			'GET /api/v1/discover/series/800001/recommendations': () => Response.json([lookupSeries(800002, 'Lantern Coast')])
		});
		renderWithClient(MoreLikeThis, { kind: 'series' as const, tmdbId: 800001 });

		const row = await screen.findByRole('region', { name: 'More like this' });
		expect(within(row).getByRole('link', { name: /Lantern Coast/ })).toHaveAttribute('href', '/shows/800002');
	});

	it('shows nothing when TMDB has no recommendations or does not know the title', async () => {
		const fetch = mockFetch({
			'GET /api/v1/discover/movies/900001/recommendations': () => Response.json([]),
			'GET /api/v1/discover/movies/123/recommendations': () => Response.json({ error: 'not found' }, { status: 404 })
		});
		renderWithClient(MoreLikeThis, { kind: 'movies' as const, tmdbId: 900001 });
		renderWithClient(MoreLikeThis, { kind: 'movies' as const, tmdbId: 123 });

		await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
		await new Promise((r) => setTimeout(r, 50));
		expect(screen.queryByRole('region')).not.toBeInTheDocument();
		expect(screen.queryByRole('alert')).not.toBeInTheDocument();
	});

	it('shows other errors', async () => {
		mockFetch({
			'GET /api/v1/discover/movies/900001/recommendations': () => Response.json({ error: 'TMDB is down' }, { status: 502 })
		});
		renderWithClient(MoreLikeThis, { kind: 'movies' as const, tmdbId: 900001 });

		expect(await screen.findByText(/TMDB is down/)).toBeInTheDocument();
	});
});
