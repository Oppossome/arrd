import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import FacetSearchBar from './FacetSearchBar.svelte';

const suggestions = [
	{ key: 'title', label: 'Search title for', value: 'anim' },
	{ key: 'anime', label: 'Anime', value: 'Japanese animation' },
	{ key: 'genre-16', label: 'Genre', value: 'Animation' }
];
const tokens = [
	{ key: 'genre', label: 'Genre', value: 'Quiet Drama' },
	{ key: 'language', label: 'Language', value: 'Quenish' }
];

function setup(props: { text?: string; tokens?: typeof tokens; suggestions?: typeof suggestions } = {}) {
	const onpick = vi.fn();
	const onremove = vi.fn();
	render(FacetSearchBar, { tokens: [], suggestions, text: 'anim', label: 'Search movies', onpick, onremove, ...props });
	return { onpick, onremove, input: screen.getByRole('combobox', { name: 'Search movies' }) };
}

const selected = () => screen.getByRole('option', { selected: true }).textContent?.replace(/\s+/g, ' ').trim();

describe('FacetSearchBar', () => {
	it('lists suggestions only while focused', async () => {
		const { input } = setup();
		expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
		await fireEvent.focus(input);
		expect(screen.getByRole('listbox')).toBeVisible();
		expect(input).toHaveAttribute('aria-expanded', 'true');
		await fireEvent.blur(input);
		expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
	});

	it('moves through suggestions with the arrow keys, wrapping, and picks with Enter', async () => {
		const { input, onpick } = setup();
		await fireEvent.focus(input);
		expect(selected()).toBe('Search title for anim');

		await fireEvent.keyDown(input, { key: 'ArrowDown' });
		await fireEvent.keyDown(input, { key: 'ArrowDown' });
		expect(selected()).toBe('Genre Animation');
		await fireEvent.keyDown(input, { key: 'ArrowDown' });
		expect(selected()).toBe('Search title for anim');
		await fireEvent.keyDown(input, { key: 'ArrowUp' });
		expect(selected()).toBe('Genre Animation');
		expect(input.getAttribute('aria-activedescendant')).toBe(screen.getByRole('option', { selected: true }).id);

		await fireEvent.keyDown(input, { key: 'Enter' });
		expect(onpick).toHaveBeenCalledWith(2);
	});

	it('picks a suggestion with the mouse', async () => {
		const { input, onpick } = setup();
		await fireEvent.focus(input);
		await fireEvent.click(screen.getByRole('option', { name: /Anime/ }));
		expect(onpick).toHaveBeenCalledWith(1);
	});

	it('closes the list on Escape until the text changes', async () => {
		const { input, onpick } = setup();
		await fireEvent.focus(input);
		await fireEvent.keyDown(input, { key: 'Escape' });
		expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
		await fireEvent.keyDown(input, { key: 'Enter' });
		expect(onpick).not.toHaveBeenCalled();

		await fireEvent.input(input, { target: { value: 'anima' } });
		expect(screen.getByRole('listbox')).toBeVisible();
	});

	it('removes tokens with their button, and the last one with Backspace in an empty input', async () => {
		const { input, onremove } = setup({ text: '', tokens, suggestions: [] });
		await fireEvent.click(screen.getByRole('button', { name: 'Remove Genre filter' }));
		expect(onremove).toHaveBeenLastCalledWith('genre');

		await fireEvent.keyDown(input, { key: 'Backspace' });
		expect(onremove).toHaveBeenLastCalledWith('language');
	});

	it('leaves tokens alone on Backspace while there is text', async () => {
		const { input, onremove } = setup({ text: 'a', tokens });
		await fireEvent.keyDown(input, { key: 'Backspace' });
		expect(onremove).not.toHaveBeenCalled();
	});
});
