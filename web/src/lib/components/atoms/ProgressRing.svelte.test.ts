import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import ProgressRing from './ProgressRing.svelte';

describe('ProgressRing', () => {
	it('reports a clamped value', () => {
		render(ProgressRing, { props: { value: 142, label: 'Download progress' } });
		const ring = screen.getByRole('progressbar', { name: 'Download progress' });
		expect(ring).toHaveAttribute('aria-valuenow', '100');
		expect(ring).toHaveTextContent('100%');
	});
});
