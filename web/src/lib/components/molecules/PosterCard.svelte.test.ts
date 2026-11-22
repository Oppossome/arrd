import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import PosterCard from './PosterCard.svelte';

const base = { href: '/library/movies/1', title: 'Lantern Bay', year: 2021 };

describe('PosterCard', () => {
	it('shows no overlay without a download', () => {
		render(PosterCard, { props: base });
		expect(screen.queryByRole('progressbar')).toBeNull();
	});

	it('shows download progress over the poster', () => {
		render(PosterCard, { props: { ...base, download: { kind: 'active', percent: 42.5, paused: false, importing: false } } });
		expect(screen.getByRole('progressbar', { name: 'Download progress' })).toHaveAttribute('aria-valuenow', '42.5');
	});

	it('marks paused, importing and failed downloads', () => {
		const { unmount } = render(PosterCard, {
			props: { ...base, download: { kind: 'active', percent: 10, paused: true, importing: false } }
		});
		expect(screen.getByRole('progressbar', { name: 'Paused' })).toBeInTheDocument();
		unmount();

		const imp = render(PosterCard, { props: { ...base, download: { kind: 'active', percent: 100, paused: false, importing: true } } });
		expect(screen.getByText('Importing')).toBeInTheDocument();
		imp.unmount();

		render(PosterCard, { props: { ...base, download: { kind: 'failed' } } });
		expect(screen.getByText('Failed')).toBeInTheDocument();
	});
});
