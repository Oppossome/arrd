import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import ConfirmDialog from './ConfirmDialog.svelte';

describe('ConfirmDialog', () => {
	it('confirms and cancels', async () => {
		const onconfirm = vi.fn();
		const oncancel = vi.fn();
		render(ConfirmDialog, { props: { open: true, title: 'Delete it?', confirmLabel: 'Delete', onconfirm, oncancel } });
		await fireEvent.click(screen.getByRole('button', { name: 'Delete' }));
		await fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
		expect(onconfirm).toHaveBeenCalledOnce();
		expect(oncancel).toHaveBeenCalledOnce();
	});
});
