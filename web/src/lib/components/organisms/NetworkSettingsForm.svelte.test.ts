import { fireEvent, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { mockFetch, renderWithClient } from '../../../test/helpers';
import NetworkSettingsForm from './NetworkSettingsForm.svelte';

afterEach(() => vi.unstubAllGlobals());

const toggleName = 'Allow access from other devices on your network';

function routes(opts: { failPut?: boolean } = {}) {
	let exposeLAN = false;
	const state = () =>
		exposeLAN
			? { expose_lan: true, hostname: 'arrd.local', lan_url: 'http://arrd.local:8484' }
			: { expose_lan: false, hostname: 'arrd.local' };
	return mockFetch({
		'GET /api/v1/settings/network': () => Response.json(state()),
		'PUT /api/v1/settings/network': (_url, init) => {
			if (opts.failPut) return Response.json({ error: 'listen on 0.0.0.0:8484: address already in use' }, { status: 500 });
			exposeLAN = JSON.parse(String(init.body)).expose_lan;
			return Response.json(state());
		}
	});
}

describe('NetworkSettingsForm', () => {
	it('turns on LAN access and shows the local address', async () => {
		const fetch = routes();
		renderWithClient(NetworkSettingsForm, {});
		const toggle = await screen.findByRole('switch', { name: toggleName });
		expect(toggle).toHaveAttribute('aria-checked', 'false');
		expect(screen.queryByRole('link')).not.toBeInTheDocument();

		await fireEvent.click(toggle);
		await waitFor(() =>
			expect(fetch.mock.calls.some((c) => c[1]?.method === 'PUT' && c[1]?.body === '{"expose_lan":true}')).toBe(true)
		);
		expect(await screen.findByRole('link', { name: 'http://arrd.local:8484' })).toHaveAttribute('href', 'http://arrd.local:8484');
		expect(screen.getByRole('switch', { name: toggleName })).toHaveAttribute('aria-checked', 'true');
	});

	it('shows why LAN access could not be turned on', async () => {
		routes({ failPut: true });
		renderWithClient(NetworkSettingsForm, {});
		await fireEvent.click(await screen.findByRole('switch', { name: toggleName }));
		expect(await screen.findByText(/address already in use/)).toBeInTheDocument();
		expect(screen.getByRole('switch', { name: toggleName })).toHaveAttribute('aria-checked', 'false');
	});
});
