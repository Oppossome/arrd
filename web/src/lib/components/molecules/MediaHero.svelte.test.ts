import { fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import MediaHero from './MediaHero.svelte';

const base = {
	title: 'The Glass Orchard',
	year: 1999,
	overview: 'A fictional orchard mystery.',
	tagline: 'Nothing grows in glass.',
	rating: 7.4,
	votes: 321,
	certification: 'PG-13',
	runtime: 118,
	genres: ['Drama', 'Mystery']
};

function prefersReducedMotion(reduce: boolean) {
	vi.stubGlobal('matchMedia', (q: string) => ({ matches: reduce && q.includes('reduce') }) as MediaQueryList);
}

beforeEach(() => vi.useFakeTimers());
afterEach(() => {
	vi.useRealTimers();
	vi.unstubAllGlobals();
});

describe('MediaHero', () => {
	it('shows the details and fades in the muted trailer', async () => {
		prefersReducedMotion(false);
		render(MediaHero, { ...base, trailerKey: 'abcXYZ01' });

		expect(screen.getByRole('heading', { name: 'The Glass Orchard' })).toBeInTheDocument();
		expect(screen.getByRole('img', { name: 'Rated 7.4 out of 10 by 321 people' })).toBeInTheDocument();
		expect(screen.getByText('PG-13')).toBeInTheDocument();
		expect(screen.getByText('1h 58m')).toBeInTheDocument();
		expect(screen.getByText('Drama · Mystery')).toBeInTheDocument();
		expect(screen.getByText('Nothing grows in glass.')).toBeInTheDocument();

		expect(screen.queryByTitle('The Glass Orchard trailer')).not.toBeInTheDocument();
		await vi.advanceTimersByTimeAsync(2000);
		const frame = screen.getByTitle('The Glass Orchard trailer');
		const src = new URL(frame.getAttribute('src')!);
		expect(src.origin).toBe('https://www.youtube-nocookie.com');
		expect(src.pathname).toBe('/embed/abcXYZ01');
		expect(src.searchParams.get('mute')).toBe('1');
		expect(src.searchParams.get('autoplay')).toBe('1');

		await fireEvent.click(screen.getByRole('button', { name: 'Unmute trailer' }));
		expect(screen.getByRole('button', { name: 'Mute trailer' })).toBeInTheDocument();
		await fireEvent.click(screen.getByRole('button', { name: 'Stop trailer' }));
		expect(screen.queryByTitle('The Glass Orchard trailer')).not.toBeInTheDocument();
	});

	it('waits for a click when the viewer prefers reduced motion', async () => {
		prefersReducedMotion(true);
		render(MediaHero, { ...base, trailerKey: 'abcXYZ01' });

		await vi.advanceTimersByTimeAsync(5000);
		expect(screen.queryByTitle('The Glass Orchard trailer')).not.toBeInTheDocument();
		await fireEvent.click(screen.getByRole('button', { name: 'Play trailer' }));
		expect(screen.getByTitle('The Glass Orchard trailer')).toBeInTheDocument();
	});

	it('has no trailer controls without a trailer', () => {
		render(MediaHero, base);
		expect(screen.queryByRole('button', { name: 'Play trailer' })).not.toBeInTheDocument();
	});
});
