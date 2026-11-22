import { screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { throughput } from '../../../test/fixtures';
import { mockFetch, renderWithClient } from '../../../test/helpers';
import ThroughputWidget from './ThroughputWidget.svelte';

afterEach(() => vi.unstubAllGlobals());

describe('ThroughputWidget', () => {
	it('stays hidden while idle', async () => {
		const fetch = mockFetch({ 'GET /api/v1/queue/throughput': () => Response.json(throughput()) });
		renderWithClient(ThroughputWidget, {});
		await vi.waitFor(() => expect(fetch).toHaveBeenCalled());
		expect(screen.queryByLabelText('Transfer rates')).toBeNull();
	});

	it('shows current and peak rates with a graph', async () => {
		const MiB = 1024 * 1024;
		mockFetch({
			'GET /api/v1/queue/throughput': () =>
				Response.json(
					throughput({
						active: 1,
						download_rate: 3 * MiB,
						upload_rate: 512,
						peak_download_rate: 9 * MiB,
						history: [
							{ at: '2026-11-22T10:00:00Z', download_rate: MiB, upload_rate: 0 },
							{ at: '2026-11-22T10:00:01Z', download_rate: 3 * MiB, upload_rate: 512 }
						]
					})
				)
		});
		renderWithClient(ThroughputWidget, {});
		expect(await screen.findByText('↓ 3.0 MiB/s')).toBeInTheDocument();
		expect(screen.getByText('↑ 512 B/s')).toBeInTheDocument();
		expect(screen.getByText('Peak 9.0 MiB/s')).toBeInTheDocument();
		expect(screen.getByRole('img', { name: 'Download rate, last 5 minutes' })).toBeInTheDocument();
	});
});
