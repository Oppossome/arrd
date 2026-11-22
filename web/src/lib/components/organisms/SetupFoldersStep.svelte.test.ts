import { screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { mockFetch, renderWithClient } from '../../../test/helpers';
import { newDraft } from '#lib/setup.js';
import SetupFoldersStep from './SetupFoldersStep.svelte';

afterEach(() => vi.unstubAllGlobals());

describe('SetupFoldersStep', () => {
	it('prefills suggestions and shows per-field errors from validation', async () => {
		mockFetch({
			'GET /api/v1/setup/status': () =>
				Response.json({
					setup_complete: false,
					suggested: { incomplete_dir: '/data/inc', complete_dir: '/data/done', movie_root: '/data/Movies', tv_root: '/data/TV' }
				}),
			'POST /api/v1/setup/validate/paths': () =>
				Response.json({ valid: false, errors: { 'downloads.complete_dir': 'must be on the same filesystem as incomplete_dir' }, warnings: [] })
		});
		const draft = $state(newDraft('tok'));
		const { instance } = renderWithClient(SetupFoldersStep, { draft });
		await waitFor(() => expect(draft.completeDir).toBe('/data/done'));
		expect(draft.movieRoot).toBe('/data/Movies');

		const ok = await instance<{ validate: () => Promise<boolean> }>().validate();
		expect(ok).toBe(false);
		expect(await screen.findByText('must be on the same filesystem as incomplete_dir')).toBeInTheDocument();
	});
});
