import { QueryClient } from '@tanstack/svelte-query';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from './api';
import { createAppQueryClient, keys, mutations, queueRefetchInterval, shouldRetry } from './query';
import type { Download, Episode, Movie } from './types';

afterEach(() => vi.unstubAllGlobals());

const invalidated = (qc: QueryClient) => {
	const spy = vi.spyOn(qc, 'invalidateQueries');
	return () => spy.mock.calls.map((c) => JSON.stringify(c[0]?.queryKey));
};

describe('query layer', () => {
	it('does not retry client errors', () => {
		expect(shouldRetry(0, new ApiError(404, 'nope'))).toBe(false);
		expect(shouldRetry(0, new ApiError(401, 'no'))).toBe(false);
		expect(shouldRetry(0, new ApiError(500, 'oops'))).toBe(true);
		expect(shouldRetry(2, new Error('network'))).toBe(false);
	});

	it('polls the queue fast only while something is in flight', () => {
		const d = (status: Download['status']) => ({ status }) as Download;
		expect(queueRefetchInterval([d('imported'), d('failed')])).toBe(15_000);
		expect(queueRefetchInterval([d('imported'), d('downloading')])).toBe(2_000);
		expect(queueRefetchInterval([d('completed')])).toBe(2_000);
		expect(queueRefetchInterval(undefined)).toBe(15_000);
	});

	it('grabbing invalidates the queue and the movie', async () => {
		const qc = new QueryClient();
		const keysOf = invalidated(qc);
		await mutations.grabMovie(qc).onSuccess({} as Download, { id: 7 });
		expect(keysOf()).toEqual(
			expect.arrayContaining([JSON.stringify(keys.queue.all), JSON.stringify(keys.movies.detail(7)), JSON.stringify(keys.movies.list)])
		);
	});

	it('VPN and network changes refresh the status banner', async () => {
		const qc = new QueryClient();
		const keysOf = invalidated(qc);
		await mutations.setVPN(qc).onSuccess();
		await mutations.updateDownloads(qc).onSuccess();
		expect(keysOf()).toEqual(expect.arrayContaining([JSON.stringify(keys.status), JSON.stringify(keys.settings.vpn)]));
	});

	it('rolls back an optimistic movie monitor toggle on error', async () => {
		const qc = new QueryClient();
		const movie = { id: 1, monitored: true } as Movie;
		qc.setQueryData(keys.movies.detail(1), movie);
		const m = mutations.setMovieMonitored(qc);
		const ctx = await m.onMutate({ id: 1, monitored: false });
		expect(qc.getQueryData<Movie>(keys.movies.detail(1))?.monitored).toBe(false);
		m.onError(new Error('x'), { id: 1 }, ctx);
		expect(qc.getQueryData<Movie>(keys.movies.detail(1))?.monitored).toBe(true);
	});

	it('toggles a whole season optimistically', async () => {
		const qc = new QueryClient();
		const eps = [
			{ id: 1, season: 1, monitored: true },
			{ id: 2, season: 2, monitored: true }
		] as Episode[];
		qc.setQueryData(keys.series.episodes(9), eps);
		const m = mutations.setSeasonMonitored(qc);
		const ctx = await m.onMutate({ seriesId: 9, season: 1, monitored: false });
		expect(qc.getQueryData<Episode[]>(keys.series.episodes(9))?.map((e) => e.monitored)).toEqual([false, true]);
		m.onError(new Error('x'), { seriesId: 9 }, ctx);
		expect(qc.getQueryData<Episode[]>(keys.series.episodes(9))).toEqual(eps);
	});

	it('routes auth and setup errors to the app hooks', async () => {
		const hooks = { onUnauthorized: vi.fn(), onSetupRequired: vi.fn(), onMutationError: vi.fn() };
		const qc = createAppQueryClient(hooks);
		await qc.fetchQuery({ queryKey: ['x'], queryFn: () => Promise.reject(new ApiError(401, 'not signed in')) }).catch(() => {});
		expect(hooks.onUnauthorized).toHaveBeenCalledOnce();

		const run = (err: Error) =>
			qc.getMutationCache().build(qc, { mutationFn: () => Promise.reject(err) }).execute(undefined).catch(() => {});
		await run(new ApiError(503, 'setup required: complete onboarding'));
		expect(hooks.onSetupRequired).toHaveBeenCalledOnce();
		await run(new ApiError(409, 'already grabbed'));
		expect(hooks.onMutationError).toHaveBeenCalledWith('already grabbed');
	});
});
