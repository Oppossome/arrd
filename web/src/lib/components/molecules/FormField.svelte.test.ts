import { render, screen } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { describe, expect, it } from 'vitest';
import FormField from './FormField.svelte';

const input = createRawSnippet((a: () => { id: string; 'aria-describedby'?: string }) => ({
	render: () => `<input id="${a().id}" aria-describedby="${a()['aria-describedby'] ?? ''}" />`
}));

describe('FormField', () => {
	it('labels the control and describes it with the error', () => {
		render(FormField, { props: { id: 'f', label: 'Folder', hint: 'a hint', error: 'does not exist', children: input } });
		const control = screen.getByLabelText('Folder');
		expect(control).toHaveAttribute('aria-describedby', 'f-error');
		expect(screen.getByText('does not exist')).toBeInTheDocument();
		expect(screen.queryByText('a hint')).not.toBeInTheDocument();
	});
	it('shows the hint without an error', () => {
		render(FormField, { props: { id: 'g', label: 'Name', hint: 'a hint', children: input } });
		expect(screen.getByLabelText('Name')).toHaveAttribute('aria-describedby', 'g-hint');
	});
});
