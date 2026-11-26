import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import FacetMenu from './FacetMenu.svelte';

const facets = [
	{ value: 'Bluray-1080p', count: 13 },
	{ value: 'HDTV-720p', count: 2 }
];

const setup = (selected: string[] = [], onclear?: () => void) => {
	const ontoggle = vi.fn();
	render(FacetMenu, { label: 'Quality', facets, selected, ontoggle, onclear });
	return { ontoggle, trigger: screen.getByRole('button', { name: /Quality/ }) };
};

describe('FacetMenu', () => {
	it('starts closed and opens to list each value with its count', async () => {
		const { trigger } = setup();
		expect(trigger).toHaveAttribute('aria-expanded', 'false');
		expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();

		await fireEvent.click(trigger);
		expect(trigger).toHaveAttribute('aria-expanded', 'true');
		expect(screen.getByRole('checkbox', { name: /Bluray-1080p\s*13/ })).not.toBeChecked();
		expect(screen.getByRole('checkbox', { name: /HDTV-720p\s*2/ })).toBeInTheDocument();
	});

	it('reports a toggled value and stays open', async () => {
		const { ontoggle, trigger } = setup();
		await fireEvent.click(trigger);
		await fireEvent.click(screen.getByRole('checkbox', { name: /HDTV-720p/ }));
		expect(ontoggle).toHaveBeenCalledExactlyOnceWith('HDTV-720p');
		expect(trigger).toHaveAttribute('aria-expanded', 'true');
	});

	it('shows how many values are selected and offers Clear', async () => {
		const onclear = vi.fn();
		const { trigger } = setup(['Bluray-1080p'], onclear);
		expect(trigger).toHaveTextContent('(1)');

		await fireEvent.click(trigger);
		expect(screen.getByRole('checkbox', { name: /Bluray-1080p/ })).toBeChecked();
		await fireEvent.click(screen.getByRole('button', { name: 'Clear' }));
		expect(onclear).toHaveBeenCalledOnce();
	});

	it('offers no Clear when nothing is selected', async () => {
		const { trigger } = setup([], vi.fn());
		await fireEvent.click(trigger);
		expect(screen.queryByRole('button', { name: 'Clear' })).not.toBeInTheDocument();
	});

	it('closes on Escape, returning focus to the trigger', async () => {
		const { trigger } = setup();
		await fireEvent.click(trigger);
		await fireEvent.keyDown(screen.getByRole('checkbox', { name: /HDTV-720p/ }), { key: 'Escape' });
		expect(trigger).toHaveAttribute('aria-expanded', 'false');
		expect(trigger).toHaveFocus();
	});

	it('closes when something outside its panel scrolls, but not for its own list', async () => {
		const { trigger } = setup();
		await fireEvent.click(trigger);
		await fireEvent.scroll(screen.getByRole('list'));
		expect(trigger).toHaveAttribute('aria-expanded', 'true');

		await fireEvent.scroll(document.body);
		expect(trigger).toHaveAttribute('aria-expanded', 'false');
	});

	it('closes on a press outside the menu', async () => {
		const { trigger } = setup();
		await fireEvent.click(trigger);
		await fireEvent.pointerDown(document.body);
		expect(trigger).toHaveAttribute('aria-expanded', 'false');
	});
});
