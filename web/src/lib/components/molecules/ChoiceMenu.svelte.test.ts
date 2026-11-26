import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import ChoiceMenu from './ChoiceMenu.svelte';

const options = [
	{ value: 'rank', label: 'Best match' },
	{ value: 'age', label: 'Newest' }
];

describe('ChoiceMenu', () => {
	it('shows the current choice, and picking another closes the menu on it', async () => {
		render(ChoiceMenu, { label: 'Sort', value: 'rank', options });

		const trigger = screen.getByRole('button', { name: 'Sort: Best match' });
		expect(trigger).toHaveAttribute('aria-expanded', 'false');

		await fireEvent.click(trigger);
		expect(screen.getByRole('button', { name: 'Best match' })).toHaveAttribute('aria-pressed', 'true');
		await fireEvent.click(screen.getByRole('button', { name: 'Newest' }));

		const updated = screen.getByRole('button', { name: 'Sort: Newest' });
		expect(updated).toHaveAttribute('aria-expanded', 'false');
		expect(updated).toHaveFocus();
	});
});
