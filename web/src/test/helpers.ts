import { QueryClient } from '@tanstack/svelte-query';
import { render } from '@testing-library/svelte';
import type { Component } from 'svelte';
import { vi } from 'vitest';
import QueryHarness from './QueryHarness.svelte';

export function testClient(): QueryClient {
	return new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
}

/** Renders an organism inside a QueryClientProvider. */
export function renderWithClient<P extends Record<string, unknown>>(component: Component<P>, props: P, client = testClient()) {
	let instance: unknown;
	const result = render(
		QueryHarness as Component<{ client: QueryClient; component: Component<P>; props: P; onref: (i: unknown) => void }>,
		{ props: { client, component, props, onref: (i) => (instance = i) } }
	);
	return { ...result, client, instance: <T>() => instance as T };
}

type Route = (url: URL, init: RequestInit) => Response | Promise<Response>;

/**
 * Stubs fetch with handlers keyed by "METHOD /path"; unknown calls fail the
 * test. The returned mock records each call as (path + query, init) with a
 * string body, whether the caller passed a URL or a Request.
 */
export function mockFetch(routes: Record<string, Route>) {
	const fn = vi.fn(async (input: string, init: RequestInit = {}) => {
		const url = new URL(input, 'http://arrd.local');
		const key = `${init.method ?? 'GET'} ${url.pathname}`;
		const handler = routes[key];
		if (!handler) throw new Error(`unexpected request ${key}`);
		return handler(url, init);
	});
	vi.stubGlobal('fetch', async (input: string | Request, init?: RequestInit) => {
		if (typeof input === 'string') return fn(input, init);
		const url = new URL(input.url);
		const body = input.body ? await input.text() : undefined;
		return fn(url.pathname + url.search, { method: input.method, headers: Object.fromEntries(input.headers), body });
	});
	return fn;
}
