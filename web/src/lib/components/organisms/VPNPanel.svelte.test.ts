import { fireEvent, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { mockFetch, renderWithClient } from '../../../test/helpers';
import type { VPNProvider, VPNSettings } from '#lib/api/types.js';
import VPNPanel from './VPNPanel.svelte';

afterEach(() => vi.unstubAllGlobals());

const providerDefs: VPNProvider[] = [
	{
		id: 'wireguard',
		name: 'WireGuard config',
		renews: false,
		fields: [{ key: 'config', label: 'WireGuard config', kind: 'textarea', required: true, secret: true, file_accept: '.conf' }]
	},
	{
		id: 'pia',
		name: 'Private Internet Access',
		renews: true,
		fields: [
			{ key: 'username', label: 'Username', kind: 'text', required: true },
			{ key: 'password', label: 'Password', kind: 'password', required: true, secret: true },
			{ key: 'region', label: 'Region', kind: 'select', required: true, options: true },
			{ key: 'port_forward', label: 'Forward a port', kind: 'toggle' }
		]
	}
];

function vpnRoutes(settings: () => VPNSettings, onPut: (body: unknown) => VPNSettings) {
	return mockFetch({
		'GET /api/v1/vpn/providers': () => Response.json(providerDefs),
		'GET /api/v1/vpn/providers/pia/options/region': () =>
			Response.json([
				{ value: 'zz-bay', label: 'Fictional Bay (port forwarding)', tags: ['port_forward'] },
				{ value: 'yy-ridge', label: 'Fictional Ridge' }
			]),
		'GET /api/v1/settings/vpn': () => Response.json(settings()),
		'PUT /api/v1/settings/vpn': (_url, init) => Response.json(onPut(JSON.parse(init.body as string)))
	});
}

describe('VPNPanel', () => {
	it('sets up a provider from its declared fields', async () => {
		let saved: VPNSettings = { configured: false, tunnel_up: false };
		const fetch = vpnRoutes(
			() => saved,
			() => {
				saved = { configured: true, provider: 'pia', tunnel_up: true, server: 'fictional101', forwarded_port: 47123, values: {}, secrets_set: ['password'] };
				return saved;
			}
		);
		renderWithClient(VPNPanel, {});
		await fireEvent.click(await screen.findByRole('button', { name: 'Set up a VPN' }));
		await fireEvent.change(screen.getByLabelText('Provider'), { target: { value: 'pia' } });

		const save = screen.getByRole('button', { name: 'Save and connect' });
		expect(save).toBeDisabled();
		await fireEvent.input(screen.getByLabelText('Username'), { target: { value: 'p0000001' } });
		await fireEvent.input(screen.getByLabelText('Password'), { target: { value: 'fictional-password-1' } });
		await screen.findByRole('option', { name: 'Fictional Bay (port forwarding)' });
		await fireEvent.change(screen.getByLabelText('Region'), { target: { value: 'zz-bay' } });
		await fireEvent.click(screen.getByRole('switch', { name: 'Forward a port' }));
		expect(save).toBeEnabled();
		await fireEvent.click(save);

		await waitFor(() => expect(fetch).toHaveBeenCalledWith('/api/v1/settings/vpn', expect.objectContaining({ method: 'PUT' })));
		const put = fetch.mock.calls.find((c) => c[1]?.method === 'PUT')!;
		expect(JSON.parse((put[1] as RequestInit).body as string)).toEqual({
			provider: 'pia',
			values: { username: 'p0000001', password: 'fictional-password-1', region: 'zz-bay', port_forward: 'true' }
		});
		expect(await screen.findByText('47123')).toBeInTheDocument();
		expect(screen.getByText('fictional101')).toBeInTheDocument();
		expect(screen.getByText(/renews this connection on its own/)).toBeInTheDocument();
	});

	it('keeps a saved secret unless the provider changes', async () => {
		const saved: VPNSettings = {
			configured: true,
			provider: 'pia',
			tunnel_up: true,
			values: { username: 'p0000001', region: 'yy-ridge', port_forward: 'false' },
			secrets_set: ['password']
		};
		const puts: unknown[] = [];
		vpnRoutes(
			() => saved,
			(body) => {
				puts.push(body);
				return saved;
			}
		);
		renderWithClient(VPNPanel, {});
		await fireEvent.click(await screen.findByRole('button', { name: 'Change VPN' }));
		expect(screen.getByLabelText('Username')).toHaveValue('p0000001');
		expect(screen.getByLabelText('Password')).toHaveValue('');
		expect(screen.getByLabelText('Password')).toHaveAttribute('placeholder', 'Saved — leave blank to keep');

		await screen.findByRole('option', { name: 'Fictional Ridge' });
		await fireEvent.click(screen.getByRole('button', { name: 'Save and connect' }));
		await waitFor(() => expect(puts).toHaveLength(1));
		expect(puts[0]).toEqual({ provider: 'pia', values: { username: 'p0000001', region: 'yy-ridge', port_forward: 'false' } });

		// A different provider starts empty: its own secret is required.
		await fireEvent.click(await screen.findByRole('button', { name: 'Change VPN' }));
		await fireEvent.change(screen.getByLabelText('Provider'), { target: { value: 'wireguard' } });
		expect(screen.getByLabelText('WireGuard config')).toHaveValue('');
		expect(screen.getByRole('button', { name: 'Save and connect' })).toBeDisabled();
		expect(screen.getByText('WireGuard config is required.')).toBeInTheDocument();
	});

	it('warns that removing the VPN exposes the real connection', async () => {
		let saved: VPNSettings = { configured: true, provider: 'wireguard', tunnel_up: true, values: {}, secrets_set: ['config'] };
		const fetch = vpnRoutes(
			() => saved,
			() => saved
		);
		const removed = vi.fn(() => {
			saved = { configured: false, tunnel_up: false };
			return new Response(null, { status: 204 });
		});
		const routes = fetch.getMockImplementation()!;
		fetch.mockImplementation(async (input: string, init: RequestInit = {}) =>
			(init.method === 'DELETE' ? removed() : routes(input, init))
		);
		renderWithClient(VPNPanel, {});

		await fireEvent.click(await screen.findByRole('button', { name: 'Remove VPN' }));
		expect(screen.getByText(/trackers and peers will see your real IP address/)).toBeInTheDocument();
		expect(removed).not.toHaveBeenCalled();
		await fireEvent.click(screen.getByRole('button', { name: 'Remove anyway' }));
		await waitFor(() => expect(removed).toHaveBeenCalledOnce());
		expect(await screen.findByText(/Indexer searches and downloads use your normal connection/)).toBeInTheDocument();
	});
});
