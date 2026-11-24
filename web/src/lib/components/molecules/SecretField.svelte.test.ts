import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import SecretField from './SecretField.svelte';

describe('SecretField', () => {
	it('shows the masked hint read-only', () => {
		render(SecretField, { label: 'Fictional key', hint: 'fi****************ey' });
		const input = screen.getByLabelText('Fictional key');
		expect(input).toHaveValue('fi****************ey');
		expect(input).toHaveAttribute('readonly');
	});

	it('says when nothing is set', () => {
		render(SecretField, { label: 'Fictional key' });
		expect(screen.queryByLabelText('Fictional key')).not.toBeInTheDocument();
		expect(screen.getByText('Not set')).toBeInTheDocument();
	});
});
