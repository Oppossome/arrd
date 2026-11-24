import { screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { mockFetch, renderWithClient } from '../../../test/helpers';
import GenreChips from './GenreChips.svelte';

afterEach(() => vi.unstubAllGlobals());

describe('GenreChips', () => {
	it('links each genre to the shows filtered by it', async () => {
		mockFetch({
			'GET /api/v1/discover/series/genres': () =>
				Response.json([
					{ id: 201, name: 'Harbor Mysteries' },
					{ id: 202, name: 'Quiet Comedies' }
				])
		});
		renderWithClient(GenreChips, { kind: 'series' as const });

		const nav = await screen.findByRole('navigation', { name: 'Browse by genre' });
		expect(within(nav).getAllByRole('link').map((a) => [a.textContent, a.getAttribute('href')])).toEqual([
			['Harbor Mysteries', '/shows?genre=201'],
			['Quiet Comedies', '/shows?genre=202']
		]);
	});
});
