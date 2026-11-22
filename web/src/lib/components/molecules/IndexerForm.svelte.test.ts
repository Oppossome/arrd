import { render, screen } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import type { Indexer, IndexerDefinition, IndexerInput } from '#lib/types.js';
import IndexerForm from './IndexerForm.svelte';

const siteAddress = (value: string) => ({ value, label: value });

const exampleBay: IndexerDefinition = {
	id: 'examplebay',
	name: 'Example Bay',
	description: 'A fictional private tracker.',
	rss: false,
	fields: [
		{
			key: 'base_url',
			label: 'Site address',
			kind: 'select',
			required: true,
			default: 'https://examplebay.invalid/',
			choices: [siteAddress('https://examplebay.invalid/'), siteAddress('https://mirror.examplebay.invalid/')]
		},
		{ key: 'username', label: 'Username', kind: 'text', required: true },
		{ key: 'password', label: 'Password', kind: 'password', required: true, secret: true }
	]
};

const openSite: IndexerDefinition = {
	id: 'opensite',
	name: 'Open Site',
	rss: true,
	fields: [{ key: 'base_url', label: 'Site address', kind: 'select', required: true, default: 'https://open.invalid/', choices: [siteAddress('https://open.invalid/')] }]
};

describe('IndexerForm', () => {
	it('builds the settings form from the site', async () => {
		const onsubmit = vi.fn<(input: IndexerInput) => void>();
		render(IndexerForm, { props: { definition: exampleBay, onsubmit } });

		// The description is shown on the site's row, not repeated in its form.
		expect(screen.queryByText('A fictional private tracker.')).not.toBeInTheDocument();
		// Fixed choices with the definition's default address preselected.
		expect(screen.getByLabelText('Site address')).toHaveValue('https://examplebay.invalid/');
		// This site cannot list recent releases.
		expect(screen.getByRole('checkbox', { name: /Use for RSS/ })).toBeDisabled();
		// Neither a name nor a network is asked for.
		expect(screen.queryByLabelText('Name')).not.toBeInTheDocument();
		expect(screen.queryByLabelText('Network')).not.toBeInTheDocument();

		const submit = screen.getByRole('button', { name: 'Switch on' });
		expect(submit).toBeDisabled(); // credentials missing
		expect(screen.getByText('Username is required.')).toBeInTheDocument();
		await userEvent.type(screen.getByLabelText('Username'), 'tester');
		await userEvent.type(screen.getByLabelText('Password'), 'fictional-pass');
		await userEvent.click(submit);
		expect(onsubmit).toHaveBeenCalledWith({
			definition: 'examplebay',
			settings: { base_url: 'https://examplebay.invalid/', username: 'tester', password: 'fictional-pass' },
			movie_categories: [2000],
			tv_categories: [5000],
			enable_rss: false,
			enable_search: true,
			priority: 25,
			min_seeders: 1
		});
	});

	it('offers RSS for sites that can list recent releases', () => {
		render(IndexerForm, { props: { definition: openSite, onsubmit: vi.fn() } });
		expect(screen.getByRole('checkbox', { name: 'Use for RSS' })).toBeEnabled();
		expect(screen.getByRole('button', { name: 'Switch on' })).toBeEnabled();
	});

	it('keeps saved secrets of the edited indexer', async () => {
		const onsubmit = vi.fn<(input: IndexerInput) => void>();
		const indexer: Indexer = {
			id: 3,
			name: 'Example Bay',
			definition: 'examplebay',
			settings: { base_url: 'https://mirror.examplebay.invalid/', username: 'tester' },
			secrets_set: ['password'],
			movie_categories: [2000],
			tv_categories: [5000],
			enabled: true,
			enable_rss: false,
			enable_search: true,
			priority: 25,
			min_seeders: 1
		};
		render(IndexerForm, { props: { definition: exampleBay, indexer, onsubmit } });
		expect(screen.getByLabelText('Site address')).toHaveValue('https://mirror.examplebay.invalid/');
		expect(screen.getByLabelText('Password')).toHaveAttribute('placeholder', 'Saved — leave blank to keep');
		await userEvent.click(screen.getByRole('button', { name: 'Save' }));
		// The blank secret is left out, so the server keeps the saved one.
		expect(onsubmit.mock.calls[0][0].settings).toEqual({ base_url: 'https://mirror.examplebay.invalid/', username: 'tester' });
	});
});
