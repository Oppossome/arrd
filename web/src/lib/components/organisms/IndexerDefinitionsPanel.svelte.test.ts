import { fireEvent, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { mockFetch, renderWithClient } from '../../../test/helpers';
import type { IndexerDefinition } from '#lib/types.js';
import IndexerDefinitionsPanel from './IndexerDefinitionsPanel.svelte';

afterEach(() => vi.unstubAllGlobals());

const exampleBay: IndexerDefinition = { id: 'examplebay', name: 'Example Bay', fields: [], rss: true };
const otherSite: IndexerDefinition = { id: 'othersite', name: 'Other Site', fields: [], rss: false };

describe('IndexerDefinitionsPanel', () => {
	it('uploads a zip as the raw request body and lists what was skipped', async () => {
		let defs: IndexerDefinition[] = [];
		const fetch = mockFetch({
			'GET /api/v1/indexers/definitions': () => Response.json(defs),
			'POST /api/v1/indexers/definitions': () => {
				defs = [exampleBay, otherSite];
				return Response.json({ saved: ['examplebay', 'othersite'], errors: ['v11/broken.yml: no id'], definitions: defs });
			}
		});
		const { container } = renderWithClient(IndexerDefinitionsPanel, {});
		await waitFor(() => expect(fetch).toHaveBeenCalledWith('/api/v1/indexers/definitions', expect.anything()));

		const input = container.querySelector('input[type=file]') as HTMLInputElement;
		const zip = new File(['PK\x03\x04fictional'], 'definitions.zip', { type: 'application/zip' });
		await fireEvent.change(input, { target: { files: [zip] } });

		await screen.findByText('1 file(s) skipped');
		expect(screen.getByText('v11/broken.yml: no id')).toBeInTheDocument();
		await screen.findByText('2 site definitions loaded');
		const init = fetch.mock.calls.find(([, i]) => i?.method === 'POST')?.[1] ?? {};
		expect(new Headers(init.headers).get('content-type')).toBe('application/octet-stream');
		expect(init.body).toBe('PK\x03\x04fictional');
	});
});
