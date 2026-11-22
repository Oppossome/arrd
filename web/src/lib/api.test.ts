import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError, api, fieldError, posterURL, toApiError } from './api';

// The client resolves API paths against the page origin.
beforeEach(() => vi.stubGlobal('location', { origin: 'http://arrd.test' }));
afterEach(() => vi.unstubAllGlobals());

/** The i-th request the fetch mock received. */
const sent = (fetchMock: ReturnType<typeof vi.fn>, i = 0) => fetchMock.mock.calls[i][0] as Request;
const pathOf = (req: Request) => {
	const u = new URL(req.url);
	return u.pathname + u.search;
};

describe('api client', () => {
	it('maps JSON errors to ApiError with status and field', async () => {
		const err = await toApiError(new Response(JSON.stringify({ error: 'is required', field: 'tmdb_api_key' }), { status: 400 }));
		expect(err).toBeInstanceOf(ApiError);
		expect(err.status).toBe(400);
		expect(err.field).toBe('tmdb_api_key');
		expect(fieldError(err, 'tmdb_api_key')).toBe('is required');
		expect(fieldError(err, 'other')).toBeUndefined();
	});
	it('keeps the status text for non-JSON errors and detects setup-required', async () => {
		const plain = await toApiError(new Response('boom', { status: 502, statusText: 'Bad Gateway' }));
		expect(plain.message).toBe('Bad Gateway');
		const setup = await toApiError(new Response(JSON.stringify({ error: 'setup required: complete onboarding' }), { status: 503 }));
		expect(setup.setupRequired).toBe(true);
	});
	it('sends JSON with same-origin credentials and handles 204', async () => {
		const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
		vi.stubGlobal('fetch', fetchMock);
		await expect(api.login('pw')).resolves.toBeUndefined();
		const req = sent(fetchMock);
		expect(pathOf(req)).toBe('/api/v1/auth/login');
		expect(req.method).toBe('POST');
		expect(req.credentials).toBe('same-origin');
		expect(req.headers.get('Content-Type')).toBe('application/json');
		expect(await req.text()).toBe(JSON.stringify({ password: 'pw' }));
	});
	it('throws ApiError for error responses', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(Response.json({ error: 'movie not found' }, { status: 404 })));
		const err: unknown = await api.movies.get(9).catch((e: unknown) => e);
		expect(err).toBeInstanceOf(ApiError);
		expect((err as ApiError).status).toBe(404);
		expect((err as ApiError).message).toBe('movie not found');
	});
	it('fills path parameters, builds query strings and poster URLs', async () => {
		const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(Response.json([])));
		vi.stubGlobal('fetch', fetchMock);
		await api.queue.list({ movieId: 3 });
		expect(pathOf(sent(fetchMock, 0))).toBe('/api/v1/queue?movie_id=3');
		await api.vpnProviders.options('a b', 'region');
		expect(pathOf(sent(fetchMock, 1))).toBe('/api/v1/vpn/providers/a%20b/options/region');
		expect(posterURL('/abc.jpg', 'w185')).toBe('/api/v1/posters/w185/abc.jpg');
		expect(posterURL(undefined)).toBeUndefined();
	});
	it('sends the setup token only to routes the wizard uses', async () => {
		const store = new Map([['arrd.setupToken', 'tok-fictional']]);
		vi.stubGlobal('sessionStorage', { getItem: (k: string) => store.get(k) ?? null });
		const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(Response.json([])));
		vi.stubGlobal('fetch', fetchMock);
		const tokenSent = (i: number) => sent(fetchMock, i).headers.get('X-Setup-Token');

		await api.vpnProviders.list();
		await api.vpnProviders.options('pia', 'region');
		await api.listDirs('/');
		await api.queue.list();
		expect([0, 1, 2, 3].map(tokenSent)).toEqual(['tok-fictional', 'tok-fictional', 'tok-fictional', null]);
	});
});
