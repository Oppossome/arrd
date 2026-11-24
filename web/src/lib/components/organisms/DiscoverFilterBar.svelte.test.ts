import { fireEvent, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { mockFetch, renderWithClient } from '../../../test/helpers';
import DiscoverFilterBar from './DiscoverFilterBar.svelte';

afterEach(() => vi.unstubAllGlobals());

const routes = () =>
	mockFetch({
		'GET /api/v1/discover/movies/genres': () => Response.json([{ id: 16, name: 'Animation' }, { id: 101, name: 'Quiet Drama' }]),
		'GET /api/v1/discover/languages': () => Response.json([{ code: 'ja', name: 'Japanese' }, { code: 'qq', name: 'Quenish' }])
	});

describe('DiscoverFilterBar', () => {
	it('applies anime when "anime" is typed and Enter pressed', async () => {
		routes();
		const onchange = vi.fn();
		renderWithClient(DiscoverFilterBar, { kind: 'movies' as const, facets: { genres: [] }, onchange });
		const input = screen.getByRole('combobox', { name: 'Search movies' });
		await fireEvent.focus(input);
		await fireEvent.input(input, { target: { value: 'anime' } });
		await screen.findByRole('option', { name: /Japanese animation/ });
		await fireEvent.keyDown(input, { key: 'Enter' });
		expect(onchange).toHaveBeenCalledWith({ genres: [16], language: 'ja' });
		expect(input).toHaveValue('');
	});

	it('applies a suggestion clicked in the list, keeping other filters', async () => {
		routes();
		const onchange = vi.fn();
		renderWithClient(DiscoverFilterBar, { kind: 'movies' as const, facets: { genres: [101] }, onchange });
		const input = screen.getByRole('combobox', { name: 'Search movies' });
		await fireEvent.focus(input);
		await fireEvent.input(input, { target: { value: 'quen' } });
		await fireEvent.click(await screen.findByRole('option', { name: /Quenish/ }));
		expect(onchange).toHaveBeenCalledWith({ genres: [101], language: 'qq' });
	});

	it('removes a token', async () => {
		routes();
		const onchange = vi.fn();
		renderWithClient(DiscoverFilterBar, { kind: 'movies' as const, facets: { genres: [101], language: 'qq' }, onchange });
		await fireEvent.click(await screen.findByRole('button', { name: 'Remove Language filter' }));
		expect(onchange).toHaveBeenCalledWith({ genres: [101], language: undefined });
	});
});
