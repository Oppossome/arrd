import { describe, expect, it } from 'vitest';
import type { VPNProvider } from './types';
import { initialValues, missingField, needsSettings, optionFields, toVPNInput } from './settingsForm';

const fictional: VPNProvider = {
	id: 'fictional',
	name: 'Fictional VPN',
	renews: true,
	fields: [
		{ key: 'user', label: 'User', kind: 'text', required: true },
		{ key: 'pass', label: 'Pass', kind: 'password', required: true, secret: true },
		{ key: 'region', label: 'Region', kind: 'select', required: true, options: true },
		{ key: 'fast', label: 'Fast', kind: 'toggle' }
	]
};

describe('vpn form helpers', () => {
	it('starts toggles off and keeps saved values', () => {
		expect(initialValues(fictional)).toEqual({ user: '', pass: '', region: '', fast: 'false' });
		expect(initialValues(fictional, { user: 'ann', fast: 'true' })).toMatchObject({ user: 'ann', fast: 'true', pass: '' });
	});

	it('lists fields that load options', () => {
		expect(optionFields(fictional).map((f) => f.key)).toEqual(['region']);
		expect(optionFields(undefined)).toEqual([]);
	});

	it('finds the first missing required field, counting saved secrets as filled', () => {
		const v = { user: 'ann', pass: '', region: 'zz', fast: 'false' };
		expect(missingField(fictional, v)?.key).toBe('pass');
		expect(missingField(fictional, v, ['pass'])).toBeNull();
		expect(missingField(fictional, { ...v, user: '  ' }, ['pass'])?.key).toBe('user');
	});

	it('leaves empty values out of the request so saved secrets are kept', () => {
		expect(toVPNInput('fictional', { user: 'ann', pass: '', fast: 'false' })).toEqual({
			provider: 'fictional',
			values: { user: 'ann', fast: 'false' }
		});
	});
});

describe('needsSettings', () => {
	it('is true only when defaults leave a required field empty', () => {
		const address = { key: 'base_url', label: 'Site address', kind: 'select', required: true, default: 'https://open.invalid/' } as const;
		expect(needsSettings({ fields: [address] })).toBe(false);
		expect(needsSettings({ fields: [address, { key: 'apikey', label: 'API key', kind: 'password', secret: true }] })).toBe(false);
		expect(needsSettings({ fields: [address, { key: 'username', label: 'Username', kind: 'text', required: true }] })).toBe(true);
	});
});
