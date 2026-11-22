import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import Sparkline from './Sparkline.svelte';

describe('Sparkline', () => {
	it('plots one point per value, scaled to max', () => {
		const { container } = render(Sparkline, { props: { values: [0, 50, 100], max: 200, label: 'Rate' } });
		expect(screen.getByRole('img', { name: 'Rate' })).toBeInTheDocument();
		expect(container.querySelector('polyline')?.getAttribute('points')).toBe('0,24 50,18 100,12');
	});

	it('draws nothing without values', () => {
		const { container } = render(Sparkline, { props: { values: [], label: 'Rate' } });
		expect(container.querySelector('polyline')).toBeNull();
	});
});
