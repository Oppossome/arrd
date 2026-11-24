import { fireEvent, screen } from '@testing-library/svelte';
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

describe('MovieActions', () => {
	it('lets the switch speak for an unmonitored movie instead of a duplicate badge', async () => {
		mockFetch({ 'GET /api/v1/movies/3': () => Response.json(movie({ monitored: false, status: 'unmonitored' })) });
		renderWithClient(MovieActions, { id: 3, ondownload: vi.fn() });

		expect(await screen.findByRole('switch', { name: 'Monitor this movie' })).not.toBeChecked();
		expect(screen.getByText('Monitored')).toBeInTheDocument();
		expect(screen.queryByText('Unmonitored')).not.toBeInTheDocument();
	});

	it('shows the status badge for a monitored movie', async () => {
		mockFetch({ 'GET /api/v1/movies/3': () => Response.json(movie({ monitored: true, status: 'missing' })) });
		renderWithClient(MovieActions, { id: 3, ondownload: vi.fn() });

		expect(await screen.findByText('Missing')).toBeInTheDocument();
	});

	it('asks for confirmation from the delete icon button', async () => {
		mockFetch({ 'GET /api/v1/movies/3': () => Response.json(movie({ monitored: true, status: 'missing' })) });
		renderWithClient(MovieActions, { id: 3, ondownload: vi.fn() });

		await fireEvent.click(await screen.findByRole('button', { name: 'Delete movie' }));
		expect(await screen.findByText('Delete The Lantern Ferry?')).toBeInTheDocument();
	});
});
