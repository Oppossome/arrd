import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import DirInput from './DirInput.svelte';

describe('DirInput', () => {
	it('browses with the injected lister and picks a folder', async () => {
		const listDirs = vi.fn((path: string) => Promise.resolve({
			path,
			parent: path === '/' ? undefined : '/',
			dirs: path === '/' ? ['media', 'srv'] : []
		}));
		const user = userEvent.setup();
		render(DirInput, { props: { value: '', listDirs } });
		await user.click(screen.getByRole('button', { name: 'Browse folders' }));
		expect(listDirs).toHaveBeenCalledWith('/');
		await user.click(await screen.findByRole('button', { name: /media/ }));
		await waitFor(() => expect(screen.getByRole('textbox')).toHaveValue('/media'));
		expect(listDirs).toHaveBeenLastCalledWith('/media');
		expect(await screen.findByText('No subfolders')).toBeInTheDocument();
	});
});

describe('DirInput with a folder that does not exist yet', () => {
	it('opens the nearest existing parent instead of failing', async () => {
		const listDirs = vi.fn((path: string) =>
			path === '/srv' ? Promise.resolve({ path, parent: '/', dirs: ['media'] }) : Promise.reject(new Error(`no such file: ${path}`))
		);
		render(DirInput, { props: { value: '/srv/arrd/downloads', listDirs } });
		await fireEvent.click(screen.getByRole('button', { name: 'Browse folders' }));
		expect(await screen.findByRole('list', { name: 'Folders in /srv' })).toBeInTheDocument();
		expect(listDirs.mock.calls.map((c) => c[0])).toEqual(['/srv/arrd/downloads', '/srv/arrd', '/srv']);
		expect(screen.queryByText(/no such file/)).not.toBeInTheDocument();
	});
});
