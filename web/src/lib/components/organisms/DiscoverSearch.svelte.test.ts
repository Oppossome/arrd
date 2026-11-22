import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import DiscoverSearchHarness from '../../../test/DiscoverSearchHarness.svelte';

describe('DiscoverSearch', () => {
	it('sets the query after a pause, once at least two characters are typed', async () => {
		render(DiscoverSearchHarness);
		const box = screen.getByLabelText('Search movies');

		await fireEvent.input(box, { target: { value: 'g' } });
		await new Promise((r) => setTimeout(r, 450));
		expect(screen.getByTestId('query')).toHaveTextContent('none');

		await fireEvent.input(box, { target: { value: '  glass ' } });
		expect(screen.getByTestId('query')).toHaveTextContent('none');
		expect(await screen.findByText('glass', {}, { timeout: 1000 })).toBeInTheDocument();
	});
});
