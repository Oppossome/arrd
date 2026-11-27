import { fireEvent, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { mockFetch, renderWithClient } from '../../../test/helpers';
import MovieActions from './MovieActions.svelte';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));

afterEach(() => vi.unstubAllGlobals());

function movie(overrides: Record<string, unknown>) {
	return {
		id: 3,
		tmdb_id: 900003,
		title: 'The Lantern Ferry',
		original_title: 'The Lantern Ferry',
		year: 2011,
		overview: 'A fictional river crossing.',
		root_folder_id: 1,
		added_at: '2026-01-01T00:00:00Z',
		...overrides
	};
}

/** The library movie, and TMDB's view of it (in collection, when given). */
function routes(m: ReturnType<typeof movie>, collection?: { id: number; name: string }) {
	return {
		'GET /api/v1/movies/3': () => Response.json(m),
		'GET /api/v1/discover/movies/900003': () =>
			Response.json({
				tmdb_id: 900003,
				title: m.title,
				original_title: m.original_title,
				year: m.year,
				overview: m.overview,
				library_id: 3,
				...(collection ? { collection } : {})
			})
	};
}

describe('MovieActions', () => {
	it('offers multi-movie packs of the movie’s collection', async () => {
		mockFetch(routes(movie({ monitored: true, status: 'missing' }), { id: 700003, name: 'Lantern Ferry Collection' }));
		const ondownload = vi.fn();
		renderWithClient(MovieActions, { id: 3, ondownload });

		await fireEvent.click(await screen.findByRole('button', { name: 'Download collection' }));
		expect(ondownload).toHaveBeenCalledWith({
			title: 'Lantern Ferry Collection · Movie packs',
			target: { kind: 'collection', tmdb_id: 900003 }
		});
	});

	it('runs an automatic search for the movie by TMDB id', async () => {
		const fetch = mockFetch({
			...routes(movie({ monitored: true, status: 'missing' })),
			'POST /api/v1/searches': () => Response.json({ grabbed: [], releases_considered: 4, indexer_failures: [] })
		});
		renderWithClient(MovieActions, { id: 3, ondownload: vi.fn() });

		await fireEvent.click(await screen.findByRole('button', { name: 'Automatic search' }));
		await waitFor(() => expect(fetch).toHaveBeenCalledWith('/api/v1/searches', expect.objectContaining({ method: 'POST' })));
		const call = fetch.mock.calls.find(([u]) => u === '/api/v1/searches')!;
		expect(JSON.parse((call[1] as RequestInit).body as string)).toEqual({ target: { kind: 'movie', tmdb_id: 900003 } });
	});

	it('offers no collection download for a standalone movie', async () => {
		const fetch = mockFetch(routes(movie({ monitored: true, status: 'missing' })));
		renderWithClient(MovieActions, { id: 3, ondownload: vi.fn() });

		expect(await screen.findByRole('button', { name: 'Download' })).toBeInTheDocument();
		await waitFor(() => expect(fetch.mock.calls.some(([u]) => String(u).includes('/discover/movies/900003'))).toBe(true));
		expect(screen.queryByRole('button', { name: 'Download collection' })).not.toBeInTheDocument();
	});

	it('lets the switch speak for an unmonitored movie instead of a duplicate badge', async () => {
		mockFetch(routes(movie({ monitored: false, status: 'unmonitored' })));
		renderWithClient(MovieActions, { id: 3, ondownload: vi.fn() });

		expect(await screen.findByRole('switch', { name: 'Monitor this movie' })).not.toBeChecked();
		expect(screen.getByText('Monitored')).toBeInTheDocument();
		expect(screen.queryByText('Unmonitored')).not.toBeInTheDocument();
	});

	it('shows the status badge for a monitored movie', async () => {
		mockFetch(routes(movie({ monitored: true, status: 'missing' })));
		renderWithClient(MovieActions, { id: 3, ondownload: vi.fn() });

		expect(await screen.findByText('Missing')).toBeInTheDocument();
	});

	it('asks for confirmation from the delete icon button', async () => {
		mockFetch(routes(movie({ monitored: true, status: 'missing' })));
		renderWithClient(MovieActions, { id: 3, ondownload: vi.fn() });

		await fireEvent.click(await screen.findByRole('button', { name: 'Delete movie' }));
		expect(await screen.findByText('Delete The Lantern Ferry?')).toBeInTheDocument();
	});
});
