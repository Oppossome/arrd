import { fireEvent, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { download, progress } from '../../../test/fixtures';
import { mockFetch, renderWithClient } from '../../../test/helpers';
import type { Movie } from '#lib/api/types.js';
import PosterGrid from './PosterGrid.svelte';

afterEach(() => vi.unstubAllGlobals());

function movie(id: number, title: string, status: Movie['status']): Movie {
	return {
		id,
		tmdb_id: 900000 + id,
		title,
		original_title: title,
		year: 2021,
		overview: '',
		root_folder_id: 1,
		monitored: true,
		added_at: '2026-11-01T00:00:00Z',
		status
	};
}

describe('PosterGrid', () => {
	it('overlays download state and filters failed items', async () => {
		mockFetch({
			'GET /api/v1/movies': () =>
				Response.json([
					movie(1, 'Lantern Bay', 'downloading'),
					movie(2, 'Copper Hollow', 'missing'),
					movie(3, 'Old Pier', 'downloaded')
				]),
			'GET /api/v1/queue': () =>
				Response.json([
					download({ movie_id: 1, progress: progress({ bytes_completed: 400, bytes_total: 1000 }) }),
					download({ movie_id: 2, status: 'failed' }),
					download({ movie_id: 3, status: 'imported' })
				])
		});
		renderWithClient(PosterGrid, { kind: 'movies' as const });

		expect(await screen.findByRole('progressbar', { name: 'Download progress' })).toHaveAttribute('aria-valuenow', '40');
		expect(screen.getByText('Failed', { selector: '.badge' })).toBeInTheDocument();

		await fireEvent.click(screen.getByRole('tab', { name: 'Failed' }));
		expect(screen.getByRole('link', { name: /Copper Hollow/ })).toBeInTheDocument();
		expect(screen.queryByRole('link', { name: /Lantern Bay/ })).toBeNull();
		expect(screen.queryByRole('link', { name: /Old Pier/ })).toBeNull();
	});
});
