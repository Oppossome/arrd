import { screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { mockFetch, renderWithClient } from '../../../test/helpers';
import { lookupMovie } from '../../../test/discover';
import DiscoverResults from './DiscoverResults.svelte';

afterEach(() => vi.unstubAllGlobals());

describe('DiscoverResults', () => {
	it('shows TMDB matches for the query', async () => {
		const fetch = mockFetch({
			'GET /api/v1/movies/lookup': () => Response.json([lookupMovie(900001, 'The Glass Orchard', 3)])
		});
		renderWithClient(DiscoverResults, { kind: 'movies' as const, query: 'glass' });

		expect(screen.getByText('Results for “glass”')).toBeInTheDocument();
		expect(await screen.findByRole('link', { name: /The Glass Orchard/ })).toHaveAttribute('href', '/movies/900001');
		expect(screen.getByText(/In library/)).toBeInTheDocument();
		expect(String(fetch.mock.calls[0][0])).toContain('q=glass');
	});

	it('sends the filter with the query', async () => {
		const fetch = mockFetch({ 'GET /api/v1/series/lookup': () => Response.json([]) });
		renderWithClient(DiscoverResults, { kind: 'series' as const, query: 'harbor', filter: { genres: [201, 202], language: 'qq' } });

		await screen.findByText('No shows match that search.');
		expect(String(fetch.mock.calls[0][0])).toBe('/api/v1/series/lookup?q=harbor&genre=201&genre=202&lang=qq');
	});

	it('says when nothing matches', async () => {
		mockFetch({ 'GET /api/v1/series/lookup': () => Response.json([]) });
		renderWithClient(DiscoverResults, { kind: 'series' as const, query: 'nothing' });

		expect(await screen.findByText('No shows match that search.')).toBeInTheDocument();
	});
});
