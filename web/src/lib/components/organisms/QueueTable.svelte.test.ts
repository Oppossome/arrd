import { screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { mockFetch, renderWithClient } from '../../../test/helpers';
import type { Download } from '#lib/api/types.js';
import QueueTable from './QueueTable.svelte';

afterEach(() => vi.unstubAllGlobals());

describe('QueueTable', () => {
	it('renders titles, episodes and progress', async () => {
		mockFetch({
			'GET /api/v1/queue': () =>
				Response.json([
					{
						id: 1,
						series_id: 3,
						title: 'Harbor Watch',
						year: 2019,
						episodes: ['S01E01', 'S01E02'],
						info_hash: 'abc',
						release_title: 'Harbor.Watch.S01.1080p.WEB-DL.x264-QUILL',
						quality: 'WEBDL-1080p',
						size: 3 * 2 ** 30,
						status: 'downloading',
						paused: false,
						added_at: '2025-01-01T00:00:00Z',
						progress: { info_hash: 'abc', name: 'x', have_info: true, bytes_completed: 2 ** 30, bytes_total: 2 * 2 ** 30, active_peers: 4, total_peers: 9, seeders: 2, complete: false, download_rate: 0, upload_rate: 0, eta_seconds: 0, countries: [] }
					}
				] satisfies Download[])
		});
		renderWithClient(QueueTable, {});
		expect(await screen.findByRole('link', { name: 'Harbor Watch (2019)' })).toHaveAttribute('href', '/library/shows/3');
		expect(screen.getByText('S01E01, S01E02')).toBeInTheDocument();
		expect(screen.getByRole('progressbar', { name: 'Download progress' })).toHaveAttribute('aria-valuenow', '50');
		expect(screen.getByText('Downloading')).toBeInTheDocument();
	});
});
