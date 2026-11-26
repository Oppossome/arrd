import { fireEvent, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { mockFetch, renderWithClient } from '../../../test/helpers';
import type { MediaServer, MediaServerDefinition, MediaServerInput } from '#lib/api/types.js';
import MediaServersPanel from './MediaServersPanel.svelte';

afterEach(() => vi.unstubAllGlobals());

const jellyfin: MediaServerDefinition = {
	id: 'jellyfin',
	name: 'Jellyfin',
	description: 'Rescans every Jellyfin library after an import.',
	fields: [
		{ key: 'url', label: 'Server URL', kind: 'text', required: true },
		{ key: 'api_key', label: 'API key', kind: 'password', required: true, secret: true }
	]
};
const plex: MediaServerDefinition = {
	id: 'plex',
	name: 'Plex',
	fields: [
		{ key: 'url', label: 'Server URL', kind: 'text', required: true },
		{ key: 'token', label: 'Plex token', kind: 'password', required: true, secret: true }
	]
};

const den: MediaServer = {
	id: 1,
	name: 'Den',
	definition: 'jellyfin',
	settings: { url: 'http://media.invalid:8096' },
	secrets_set: ['api_key'],
	enabled: true
};

/** A fake API holding media servers in memory; returns the request log. */
function fakeAPI(opts: { servers?: MediaServer[]; testFails?: boolean } = {}) {
	let servers = opts.servers ?? [];
	const posts: MediaServerInput[] = [];
	const puts: { id: number; body: MediaServerInput }[] = [];
	const tests: number[] = [];
	const fetch = mockFetch({
		'GET /api/v1/mediaservers': () => Response.json(servers),
		'GET /api/v1/mediaservers/definitions': () => Response.json([plex, jellyfin]),
		'POST /api/v1/mediaservers': (_url, init) => {
			const body = JSON.parse(init.body as string) as MediaServerInput;
			posts.push(body);
			const def = body.definition === 'plex' ? plex : jellyfin;
			const created: MediaServer = {
				id: servers.length + 1,
				name: body.name ?? def.name,
				definition: def.id,
				settings: { url: body.settings?.url ?? '' },
				enabled: true
			};
			servers = [...servers, created];
			return Response.json(created, { status: 201 });
		},
		'PUT /api/v1/mediaservers/1': (_url, init) => {
			const body = JSON.parse(init.body as string) as MediaServerInput;
			puts.push({ id: 1, body });
			servers = servers.map((s) => (s.id === 1 ? { ...s, ...body, settings: body.settings ?? s.settings } : s));
			return Response.json(servers[0]);
		},
		'POST /api/v1/mediaservers/1/test': () => {
			tests.push(1);
			if (opts.testFails) return Response.json({ error: 'GET /System/Info: 401 Unauthorized (check the token)' }, { status: 502 });
			return Response.json(servers[0]);
		}
	});
	return { fetch, posts, puts, tests };
}

describe('MediaServersPanel', () => {
	it('adds a server of the chosen type', async () => {
		const api = fakeAPI();
		renderWithClient(MediaServersPanel, {});
		await fireEvent.click(await screen.findByRole('button', { name: 'Add media server' }));

		const form = screen.getByRole('form', { name: 'New media server' });
		await fireEvent.change(within(form).getByLabelText('Type'), { target: { value: 'jellyfin' } });
		const add = within(form).getByRole('button', { name: 'Add' });
		expect(add).toBeDisabled();
		await fireEvent.input(within(form).getByLabelText('Server URL'), { target: { value: 'http://media.invalid:8096' } });
		await fireEvent.input(within(form).getByLabelText('API key'), { target: { value: 'fictional-key' } });
		await fireEvent.click(add);

		await waitFor(() => expect(api.posts).toHaveLength(1));
		expect(api.posts[0]).toEqual({ definition: 'jellyfin', settings: { url: 'http://media.invalid:8096', api_key: 'fictional-key' } });
		expect(await screen.findByRole('switch', { name: 'Switch off Jellyfin' })).toBeInTheDocument();
		await waitFor(() => expect(screen.queryByRole('form', { name: 'New media server' })).not.toBeInTheDocument());
	});

	it('edits a server without resending its saved key', async () => {
		const api = fakeAPI({ servers: [den] });
		renderWithClient(MediaServersPanel, {});
		await fireEvent.click(await screen.findByRole('button', { name: 'Edit Den' }));

		const form = screen.getByRole('form', { name: 'Den settings' });
		// The saved key counts as filled in.
		const save = within(form).getByRole('button', { name: 'Save' });
		expect(save).toBeEnabled();
		await fireEvent.input(within(form).getByLabelText('Server URL'), { target: { value: 'http://media.invalid:9000' } });
		await fireEvent.click(save);

		await waitFor(() => expect(api.puts).toHaveLength(1));
		expect(api.puts[0].body).toEqual({ name: 'Den', definition: 'jellyfin', settings: { url: 'http://media.invalid:9000' } });
	});

	it('switches a server off', async () => {
		const api = fakeAPI({ servers: [den] });
		renderWithClient(MediaServersPanel, {});
		await fireEvent.click(await screen.findByRole('switch', { name: 'Switch off Den' }));
		await waitFor(() => expect(api.puts).toEqual([{ id: 1, body: { enabled: false } }]));
	});

	it('tests a server and shows its last error', async () => {
		const api = fakeAPI({ servers: [{ ...den, last_error: 'POST /Library/Refresh: 503 Service Unavailable' }], testFails: true });
		renderWithClient(MediaServersPanel, {});
		expect(await screen.findByText(/503 Service Unavailable/)).toBeInTheDocument();
		await fireEvent.click(screen.getByRole('button', { name: 'Test' }));
		await waitFor(() => expect(api.tests).toEqual([1]));
	});

	it('says when there are none', async () => {
		fakeAPI();
		renderWithClient(MediaServersPanel, {});
		expect(await screen.findByText('No media servers yet.')).toBeInTheDocument();
	});
});
