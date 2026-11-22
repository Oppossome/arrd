import { fireEvent, screen, waitFor } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { mockFetch, renderWithClient } from '../../../test/helpers';
import { lookupMovie } from '../../../test/discover';
import BrowseGrid from './BrowseGrid.svelte';

/** The observers watching the grid's end; jsdom has no IntersectionObserver. */
let observers: FakeObserver[] = [];
class FakeObserver {
	constructor(private callback: IntersectionObserverCallback) {}
	observe() {
		observers.push(this);
	}
	disconnect() {
		observers = observers.filter((o) => o !== this);
	}
	/** Reports the watched element as on (or near) the screen. */
	intersect() {
		this.callback([{ isIntersecting: true } as IntersectionObserverEntry], this as unknown as IntersectionObserver);
	}
}
/** Scrolls the end of the grid into view. */
const reachEnd = () => observers.forEach((o) => o.intersect());

beforeEach(() => {
	observers = [];
	vi.stubGlobal('IntersectionObserver', FakeObserver);
});
afterEach(() => vi.unstubAllGlobals());

const routes = () =>
	mockFetch({
		'GET /api/v1/discover/movies/genres': () => Response.json([{ id: 101, name: 'Quiet Drama' }]),
		'GET /api/v1/discover/movies/browse': (url) =>
			Response.json(
				url.searchParams.get('page') === '2'
					? { page: 2, total_pages: 2, items: [lookupMovie(900002, 'Paper Lanterns'), lookupMovie(900003, 'Salt Roads')] }
					: { page: 1, total_pages: 2, items: [lookupMovie(900001, 'The Glass Orchard'), lookupMovie(900002, 'Paper Lanterns')] }
			)
	});

describe('BrowseGrid', () => {
	it('loads the next page when the end of the grid comes into view, without repeating titles', async () => {
		const fetch = routes();
		renderWithClient(BrowseGrid, { kind: 'movies' as const, genre: 101, sort: 'top_rated' as const, onchange: vi.fn() });
		const browseCalls = () => fetch.mock.calls.map((c) => String(c[0])).filter((p) => p.includes('/browse'));

		expect(await screen.findByRole('link', { name: /The Glass Orchard/ })).toHaveAttribute('href', '/movies/900001');
		expect(browseCalls()).toEqual(['/api/v1/discover/movies/browse?genre=101&sort=top_rated&page=1']);
		await waitFor(() => expect(observers).toHaveLength(1));

		reachEnd();
		expect(await screen.findByRole('link', { name: /Salt Roads/ })).toBeInTheDocument();
		expect(screen.getAllByRole('link', { name: /Paper Lanterns/ })).toHaveLength(1);
		expect(browseCalls()).toEqual([
			'/api/v1/discover/movies/browse?genre=101&sort=top_rated&page=1',
			'/api/v1/discover/movies/browse?genre=101&sort=top_rated&page=2'
		]);

		// The last page: nothing left to watch for.
		await waitFor(() => expect(observers).toHaveLength(0));
	});

	it('waits for a retry after a page fails', async () => {
		let failNext = true;
		const fetch = mockFetch({
			'GET /api/v1/discover/movies/genres': () => Response.json([]),
			'GET /api/v1/discover/movies/browse': (url) => {
				if (url.searchParams.get('page') === '1')
					return Response.json({ page: 1, total_pages: 2, items: [lookupMovie(900001, 'The Glass Orchard')] });
				if (failNext) {
					failNext = false;
					return Response.json({ error: 'TMDB is down' }, { status: 502 });
				}
				return Response.json({ page: 2, total_pages: 2, items: [lookupMovie(900003, 'Salt Roads')] });
			}
		});
		renderWithClient(BrowseGrid, { kind: 'movies' as const, sort: 'popular' as const, onchange: vi.fn() });
		await screen.findByRole('link', { name: /The Glass Orchard/ });
		await waitFor(() => expect(observers).toHaveLength(1));

		reachEnd();
		expect(await screen.findByText(/TMDB is down/)).toBeInTheDocument();
		expect(observers).toHaveLength(0);
		const pageTwoCalls = () => fetch.mock.calls.filter((c) => String(c[0]).includes('page=2')).length;
		expect(pageTwoCalls()).toBe(1);

		await fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
		expect(await screen.findByRole('link', { name: /Salt Roads/ })).toBeInTheDocument();
		expect(pageTwoCalls()).toBe(2);
	});

	it('reports a new genre or order', async () => {
		routes();
		const onchange = vi.fn();
		renderWithClient(BrowseGrid, { kind: 'movies' as const, genre: 101, sort: 'popular' as const, onchange });
		await screen.findByRole('option', { name: 'Quiet Drama' });

		await fireEvent.change(screen.getByLabelText('Order'), { target: { value: 'newest' } });
		expect(onchange).toHaveBeenLastCalledWith({ genre: 101, sort: 'newest' });

		await fireEvent.change(screen.getByLabelText('Genre'), { target: { value: '0' } });
		expect(onchange).toHaveBeenLastCalledWith({ genre: undefined, sort: 'popular' });
	});

	it('shows an error when the first page fails', async () => {
		mockFetch({
			'GET /api/v1/discover/series/genres': () => Response.json([]),
			'GET /api/v1/discover/series/browse': () => Response.json({ error: 'TMDB is down' }, { status: 502 })
		});
		renderWithClient(BrowseGrid, { kind: 'series' as const, sort: 'popular' as const, onchange: vi.fn() });

		expect(await screen.findByText(/TMDB is down/)).toBeInTheDocument();
	});
});
