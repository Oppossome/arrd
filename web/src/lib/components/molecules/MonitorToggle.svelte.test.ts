import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import MonitorToggle from './MonitorToggle.svelte';

describe('MonitorToggle', () => {
	it('renders a bare switch by default', () => {
		render(MonitorToggle, { monitored: true, what: 'this movie', onchange: vi.fn() });
		expect(screen.getByRole('switch', { name: 'Stop monitoring this movie' })).toBeChecked();
		expect(screen.queryByText('Monitored')).not.toBeInTheDocument();
	});

	it('with showLabel, keeps a fixed caption and toggles when the caption is clicked', async () => {
		const onchange = vi.fn();
		render(MonitorToggle, { monitored: false, what: 'this movie', showLabel: true, onchange });

		expect(screen.getByRole('switch', { name: 'Monitor this movie' })).not.toBeChecked();
		await fireEvent.click(screen.getByText('Monitored'));
		expect(onchange).toHaveBeenCalledExactlyOnceWith(true);
	});

	it('toggles once when the switch itself is clicked inside the label', async () => {
		const onchange = vi.fn();
		render(MonitorToggle, { monitored: true, what: 'this movie', showLabel: true, onchange });

		await fireEvent.click(screen.getByRole('switch'));
		expect(onchange).toHaveBeenCalledExactlyOnceWith(false);
	});
});
