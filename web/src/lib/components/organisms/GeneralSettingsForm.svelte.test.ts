import { fireEvent, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { mockFetch, renderWithClient } from '../../../test/helpers';
import GeneralSettingsForm from './GeneralSettingsForm.svelte';

afterEach(() => vi.unstubAllGlobals());

function routes(tmdbSet: boolean) {
	return mockFetch({
		'GET /api/v1/settings/general': () => Response.json(
				tmdbSet
					? { tmdb_api_key_set: true, tmdb_api_key_hint: 'fi****************ey', api_key_hint: 'ap****************01' }
					: { tmdb_api_key_set: false, api_key_hint: 'ap****************01' }
			),
		'POST /api/v1/settings/api-key/regenerate': () => Response.json({ api_key: 'fictional-app-key-0001' })
	});
}

describe('GeneralSettingsForm', () => {
	it('masks a saved TMDB key and edits it on demand', async () => {
		routes(true);
		renderWithClient(GeneralSettingsForm, {});
		expect(await screen.findByLabelText('TMDB API key')).toHaveValue('fi****************ey');

		await fireEvent.click(screen.getByRole('button', { name: 'Replace' }));
		const input = screen.getByLabelText('TMDB API key');
		expect(input).toHaveAttribute('type', 'password');
		expect(input).toHaveValue('');

		await fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
		expect(screen.getByLabelText('TMDB API key')).toHaveValue('fi****************ey');
	});

	it('offers to add a TMDB key when none is saved', async () => {
		routes(false);
		renderWithClient(GeneralSettingsForm, {});
		expect(await screen.findByRole('button', { name: 'Add key' })).toBeInTheDocument();
		expect(screen.getByText('Not set')).toBeInTheDocument();
	});

	it('regenerates the app API key only after confirmation', async () => {
		const fetch = routes(true);
		renderWithClient(GeneralSettingsForm, {});
		expect(await screen.findByRole('textbox', { name: 'App API key' })).toHaveValue('ap****************01');

		await fireEvent.click(screen.getByRole('button', { name: 'Regenerate' }));
		const isRegenerate = (c: unknown[]) => c[0] === '/api/v1/settings/api-key/regenerate';
		expect(fetch.mock.calls.some(isRegenerate)).toBe(false);

		const dialog = screen.getByRole('dialog', { name: 'Regenerate the API key?' });
		await fireEvent.click(within(dialog).getByRole('button', { name: 'Regenerate' }));
		await waitFor(() => expect(fetch.mock.calls.some(isRegenerate)).toBe(true));
		expect(await screen.findByDisplayValue('fictional-app-key-0001')).toBeInTheDocument();

		await fireEvent.click(screen.getByRole('button', { name: 'Done' }));
		expect(screen.queryByDisplayValue('fictional-app-key-0001')).not.toBeInTheDocument();
		expect(screen.getByRole('textbox', { name: 'App API key' })).toHaveValue('ap****************01');
	});

	it('hides the password form until asked', async () => {
		routes(true);
		renderWithClient(GeneralSettingsForm, {});
		expect(screen.queryByLabelText('Current password')).not.toBeInTheDocument();

		await fireEvent.click(screen.getByRole('button', { name: 'Change password…' }));
		expect(screen.getByLabelText('Current password')).toBeInTheDocument();
		expect(screen.getByLabelText('New password')).toBeInTheDocument();
		expect(screen.getByLabelText('Confirm new password')).toBeInTheDocument();

		await fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
		expect(screen.queryByLabelText('Current password')).not.toBeInTheDocument();
	});
});
