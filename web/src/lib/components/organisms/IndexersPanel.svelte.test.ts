import { fireEvent, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { mockFetch, renderWithClient } from '../../../test/helpers';
import type { Indexer, IndexerDefinition, IndexerInput, VPNSettings } from '#lib/types.js';
import IndexersPanel from './IndexersPanel.svelte';

afterEach(() => vi.unstubAllGlobals());

const address = (value: string) => ({
	key: 'base_url',
	label: 'Site address',
	kind: 'select' as const,
	required: true,
	default: value,
	choices: [{ value, label: value }]
});

const openSite: IndexerDefinition = {
	id: 'opensite',
	name: 'Open Site',
	description: 'A fictional public tracker.',
	rss: true,
	fields: [address('https://open.invalid/')]
};
const exampleBay: IndexerDefinition = {
	id: 'examplebay',
	name: 'Example Bay',
	rss: false,
	fields: [
		address('https://examplebay.invalid/'),
		{ key: 'username', label: 'Username', kind: 'text', required: true },
		{ key: 'password', label: 'Password', kind: 'password', required: true, secret: true }
	]
};
const captchaSite: IndexerDefinition = { id: 'captchasite', name: 'Captcha Site', rss: false, fields: [], unsupported: 'signing in needs a captcha' };

function indexerFor(def: IndexerDefinition, id: number, input: IndexerInput): Indexer {
	return {
		id,
		name: def.name,
		definition: def.id,
		settings: input.settings ?? {},
		movie_categories: [2000],
		tv_categories: [5000],
		enabled: input.enabled ?? true,
		enable_rss: def.rss,
		enable_search: true,
		priority: 25,
		min_seeders: 1
	};
}

/** A fake API holding indexers in memory; returns the request log. */
function fakeAPI(opts: { indexers?: Indexer[]; vpn?: VPNSettings } = {}) {
	let indexers = opts.indexers ?? [];
	const defs = [captchaSite, exampleBay, openSite];
	const posts: IndexerInput[] = [];
	const puts: { id: number; body: IndexerInput }[] = [];
	const update = (url: URL, init: RequestInit) => {
		const id = Number(url.pathname.split('/').pop());
		const body = JSON.parse(init.body as string) as IndexerInput;
		puts.push({ id, body });
		indexers = indexers.map((ix) => (ix.id === id ? { ...ix, ...body, settings: body.settings ?? ix.settings } : ix));
		return Response.json(indexers.find((ix) => ix.id === id));
	};
	const fetch = mockFetch({
		'GET /api/v1/indexers': () => Response.json(indexers),
		'GET /api/v1/indexers/definitions': () => Response.json(defs),
		'GET /api/v1/settings/vpn': () => Response.json(opts.vpn ?? { configured: true, tunnel_up: true }),
		'POST /api/v1/indexers': (_url, init) => {
			const body = JSON.parse(init.body as string) as IndexerInput;
			posts.push(body);
			const ix = indexerFor(defs.find((d) => d.id === body.definition)!, indexers.length + 1, body);
			indexers = [...indexers, ix];
			return Response.json(ix, { status: 201 });
		},
		'PUT /api/v1/indexers/1': update,
		'PUT /api/v1/indexers/2': update
	});
	return { fetch, posts, puts };
}

describe('IndexersPanel', () => {
	it('switches on a site that needs no settings straight away', async () => {
		const api = fakeAPI();
		renderWithClient(IndexersPanel, {});
		await fireEvent.click(await screen.findByRole('switch', { name: 'Switch on Open Site' }));

		await waitFor(() => expect(api.posts).toHaveLength(1));
		expect(api.posts[0]).toEqual({ definition: 'opensite', settings: { base_url: 'https://open.invalid/' }, enabled: true });
		expect(await screen.findByRole('switch', { name: 'Switch off Open Site' })).toHaveAttribute('aria-checked', 'true');
		// An unsupported site cannot be switched on.
		expect(screen.getByRole('switch', { name: 'Switch on Captcha Site' })).toBeDisabled();
	});

	it('asks for the settings of a site that needs a sign-in before switching it on', async () => {
		const api = fakeAPI();
		renderWithClient(IndexersPanel, {});
		await fireEvent.click(await screen.findByRole('switch', { name: 'Switch on Example Bay' }));
		expect(api.posts).toHaveLength(0);

		const form = screen.getByRole('form', { name: 'Example Bay settings' });
		await fireEvent.input(within(form).getByLabelText('Username'), { target: { value: 'tester' } });
		await fireEvent.input(within(form).getByLabelText('Password'), { target: { value: 'fictional-pass' } });
		await fireEvent.click(within(form).getByRole('button', { name: 'Switch on' }));

		await waitFor(() => expect(api.posts).toHaveLength(1));
		expect(api.posts[0]).toMatchObject({
			definition: 'examplebay',
			settings: { base_url: 'https://examplebay.invalid/', username: 'tester', password: 'fictional-pass' },
			enabled: true
		});
		await waitFor(() => expect(screen.queryByRole('form', { name: 'Example Bay settings' })).not.toBeInTheDocument());
	});

	it('switches a site off and on again, keeping its settings', async () => {
		const saved = indexerFor(exampleBay, 1, { settings: { base_url: 'https://examplebay.invalid/', username: 'tester' } });
		const api = fakeAPI({ indexers: [saved] });
		renderWithClient(IndexersPanel, {});

		await fireEvent.click(await screen.findByRole('switch', { name: 'Switch off Example Bay' }));
		await waitFor(() => expect(api.puts).toEqual([{ id: 1, body: { enabled: false } }]));
		await fireEvent.click(await screen.findByRole('switch', { name: 'Switch on Example Bay' }));
		await waitFor(() => expect(api.puts).toHaveLength(2));
		// No settings are sent, so the saved ones (and the password) are kept.
		expect(api.puts[1]).toEqual({ id: 1, body: { enabled: true } });
		expect(api.posts).toHaveLength(0);
	});

	it("shows each site's description in the list", async () => {
		fakeAPI();
		renderWithClient(IndexersPanel, {});
		expect(await screen.findByText('A fictional public tracker.')).toBeInTheDocument();
	});

	it('says when traffic is not going through a VPN', async () => {
		fakeAPI({ vpn: { configured: false, tunnel_up: false } });
		renderWithClient(IndexersPanel, {});
		expect(await screen.findByText(/Indexers use your normal connection/)).toBeInTheDocument();
	});

	it('lists indexers whose definition is gone so they can be deleted', async () => {
		const orphan = { ...indexerFor(openSite, 2, {}), name: 'Gone Site', definition: 'gonesite' };
		fakeAPI({ indexers: [orphan] });
		renderWithClient(IndexersPanel, {});
		expect(await screen.findByRole('button', { name: 'Delete Gone Site' })).toBeInTheDocument();
		expect(screen.getByText(/no longer loaded/)).toBeInTheDocument();
	});
});
