import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import { download, progress } from '../../../test/fixtures';
import DownloadCard from './DownloadCard.svelte';

const handlers = () => ({ onpause: vi.fn(), onresume: vi.fn(), onremove: vi.fn(), onpeers: vi.fn() });

describe('DownloadCard', () => {
	it('shows rate, time left and the peer countries', async () => {
		const h = handlers();
		const d = download({
			title: 'Lantern Bay',
			year: 2021,
			backdrop_path: '/lanternbackdrop.jpg',
			progress: progress({
				bytes_completed: 250,
				download_rate: 2 * 1024 * 1024,
				eta_seconds: 245,
				countries: [
					{ country: 'DE', count: 3 },
					{ country: 'NZ', count: 2 },
					{ country: 'FR', count: 1 },
					{ country: '', count: 4 }
				]
			})
		});
		const { container } = render(DownloadCard, { props: { download: d, href: '/library/movies/4', ...h } });

		expect(screen.getByRole('link', { name: 'Lantern Bay' })).toHaveAttribute('href', '/library/movies/4');
		expect(screen.getByText('2.0 MiB/s · 4m 05s left')).toBeInTheDocument();
		expect(screen.getByRole('progressbar', { name: 'Download progress' })).toHaveAttribute('aria-valuenow', '25');
		expect(container.querySelector('img.art')).toHaveAttribute('src', '/api/v1/posters/w780/lanternbackdrop.jpg');
		expect(screen.getByText('+4')).toBeInTheDocument();

		await fireEvent.click(screen.getByRole('button', { name: 'Show 10 peers' }));
		await fireEvent.click(screen.getByRole('button', { name: 'Pause' }));
		expect(h.onpeers).toHaveBeenCalledOnce();
		expect(h.onpause).toHaveBeenCalledOnce();
	});

	it('offers resume while paused and blurs the poster without a backdrop', async () => {
		const h = handlers();
		const d = download({ title: 'Lantern Bay', paused: true, poster_path: '/lanternposter.jpg', progress: progress() });
		const { container } = render(DownloadCard, { props: { download: d, ...h } });

		expect(screen.getByText('Paused')).toBeInTheDocument();
		expect(container.querySelector('img.art.blurred')).toHaveAttribute('src', '/api/v1/posters/w342/lanternposter.jpg');
		await fireEvent.click(screen.getByRole('button', { name: 'Resume' }));
		expect(h.onresume).toHaveBeenCalledOnce();
		expect(screen.queryByRole('button', { name: 'Pause' })).toBeNull();
	});

	it('cannot pause a finished download that is being imported', () => {
		render(DownloadCard, { props: { download: download({ status: 'completed' }), ...handlers() } });
		expect(screen.getByText('Importing')).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Pause' })).toBeNull();
		expect(screen.getByRole('button', { name: 'Remove' })).toBeInTheDocument();
	});
});
