import { fireEvent, render, screen } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import type { VPNProvider } from '#lib/types.js';
import SettingsForm from './SettingsForm.svelte';

const fictionalVPN: VPNProvider = {
	id: 'fictional',
	name: 'Fictional VPN',
	renews: true,
	fields: [
		{ key: 'username', label: 'Username', kind: 'text', required: true },
		{ key: 'password', label: 'Password', kind: 'password', required: true, secret: true },
		{ key: 'region', label: 'Region', kind: 'select', required: true, options: true },
		{ key: 'config', label: 'Config', kind: 'textarea', file_accept: '.conf', help: 'paste it here' },
		{ key: 'port_forward', label: 'Forward a port', kind: 'toggle', help: 'helps seeding' }
	]
};

describe('SettingsForm', () => {
	it('renders every declared field kind and edits the values', async () => {
		const values = { username: '', password: '', region: '', config: '', port_forward: 'false' };
		render(SettingsForm, {
			props: {
				provider: fictionalVPN,
				values,
				options: { region: [{ value: 'zz-bay', label: 'Fictional Bay (port forwarding)' }] },
				error: { field: 'password', message: 'rejected by the provider' }
			}
		});
		expect(screen.getByLabelText('Username')).toHaveAttribute('type', 'text');
		expect(screen.getByLabelText('Password')).toHaveAttribute('type', 'password');
		expect(screen.getByText('rejected by the provider')).toBeInTheDocument();
		expect(screen.getByLabelText('Config (optional)').tagName).toBe('TEXTAREA');
		expect(screen.getByText('Or load a .conf file:')).toBeInTheDocument();
		expect(screen.getByText('helps seeding')).toBeInTheDocument();

		await userEvent.type(screen.getByLabelText('Username'), 'p0000001');
		await userEvent.selectOptions(screen.getByLabelText('Region'), 'zz-bay');
		await fireEvent.click(screen.getByRole('switch', { name: 'Forward a port' }));
		expect(values).toMatchObject({ username: 'p0000001', region: 'zz-bay', port_forward: 'true' });
	});

	it('marks saved secrets and shows a loading select', () => {
		render(SettingsForm, {
			props: { provider: fictionalVPN, values: {}, secretsSet: ['password'], loading: ['region'] }
		});
		expect(screen.getByLabelText('Password')).toHaveAttribute('placeholder', 'Saved — leave blank to keep');
		expect(screen.getByLabelText('Username')).not.toHaveAttribute('placeholder', 'Saved — leave blank to keep');
		expect(screen.getByLabelText('Region')).toBeDisabled();
		expect(screen.getByRole('option', { name: 'Loading…' })).toBeInTheDocument();
	});
});
