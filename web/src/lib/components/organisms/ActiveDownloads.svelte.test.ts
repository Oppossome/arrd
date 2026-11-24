import { fireEvent, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { download, peer, progress } from '../../../test/fixtures';
import { mockFetch, renderWithClient } from '../../../test/helpers';
import { countryName } from '#lib/utils/countries.js';
import ActiveDownloads from './ActiveDownloads.svelte';

afterEach(() => vi.unstubAllGlobals());

describe('ActiveDownloads', () => {
	it('shows nothing when no download is running', async () => {
		const fetch = mockFetch({
			'GET /api/v1/queue': () => Response.json([download({ status: 'imported' }), download({ status: 'failed' })])
		});
		renderWithClient(ActiveDownloads, {});
		await vi.waitFor(() => expect(fetch).toHaveBeenCalled());
		expect(screen.queryByRole('region', { name: 'Active now' })).toBeNull();
	});

	it('lists running downloads and pauses one', async () => {
		const running = download({ id: 7, movie_id: 4, title: 'Lantern Bay', progress: progress() });
		const fetch = mockFetch({
			'GET /api/v1/queue': () => Response.json([running, download({ status: 'imported', title: 'Old Pier' })]),
			'POST /api/v1/queue/7/pause': () => new Response(null, { status: 204 })
		});
		renderWithClient(ActiveDownloads, {});

		const shelf = await screen.findByRole('region', { name: 'Active now' });
		expect(within(shelf).getAllByRole('article')).toHaveLength(1);
		expect(within(shelf).getByRole('link', { name: 'Lantern Bay' })).toHaveAttribute('href', '/library/movies/4');

		await fireEvent.click(within(shelf).getByRole('button', { name: 'Pause' }));
		await vi.waitFor(() => expect(fetch).toHaveBeenCalledWith('/api/v1/queue/7/pause', expect.objectContaining({ method: 'POST' })));
	});

	it('opens the peer list with addresses and countries', async () => {
		const running = download({
			id: 9,
			title: 'Lantern Bay',
			progress: progress({ countries: [{ country: 'DE', count: 1 }, { country: '', count: 1 }] })
		});
		mockFetch({
			'GET /api/v1/queue': () => Response.json([running]),
			'GET /api/v1/queue/9/peers': () =>
				Response.json([
					peer({ ip: '198.51.100.7', port: 51413, country: 'DE', download_rate: 1024, progress: 0.5 }),
					peer({ ip: '2001:db8::5', port: 6881, country: '', client: '', source: 'dht' })
				])
		});
		renderWithClient(ActiveDownloads, {});

		await fireEvent.click(await screen.findByRole('button', { name: 'Show 2 peers' }));
		const dialog = await screen.findByRole('dialog', { name: 'Peers of Lantern Bay' });
		expect(await within(dialog).findByText('198.51.100.7:51413')).toBeInTheDocument();
		expect(within(dialog).getByText('[2001:db8::5]:6881')).toBeInTheDocument();
		expect(within(dialog).getByRole('img', { name: countryName('DE') })).toBeInTheDocument();
		expect(within(dialog).getByRole('img', { name: 'Unknown location' })).toBeInTheDocument();
		expect(within(dialog).getByText('DHT')).toBeInTheDocument();
		expect(within(dialog).getByText('50%')).toBeInTheDocument();
		expect(within(dialog).getByRole('link', { name: 'IP Geolocation by DB-IP' })).toHaveAttribute('href', 'https://db-ip.com');
	});
});
