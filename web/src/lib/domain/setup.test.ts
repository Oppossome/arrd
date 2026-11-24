import { describe, expect, it } from 'vitest';
import { newDraft, passwordProblem, toSetupInput, vpnProblem } from './setup';

describe('setup draft', () => {
	it('checks the account step', () => {
		const d = newDraft('tok');
		expect(passwordProblem({ ...d, token: '' })).toMatch(/setup token/);
		expect(passwordProblem({ ...d, password: 'short', passwordConfirm: 'short' })).toMatch(/at least 8/);
		expect(passwordProblem({ ...d, password: 'long enough', passwordConfirm: 'long enougH' })).toMatch(/match/);
		expect(passwordProblem({ ...d, password: 'long enough', passwordConfirm: 'long enough' })).toBeNull();
	});
	it('requires a tested config before using the VPN', () => {
		expect(vpnProblem({ vpnProvider: '', vpnTestedIP: '' })).toBeNull();
		expect(vpnProblem({ vpnProvider: 'pia', vpnTestedIP: '' })).toMatch(/Test/);
		expect(vpnProblem({ vpnProvider: 'pia', vpnTestedIP: '198.51.100.9' })).toBeNull();
	});
	it('builds the API request', () => {
		const d = newDraft('tok', { incomplete_dir: '/i', complete_dir: '/c', movie_root: '/m', tv_root: '' });
		d.password = 'correct horse';
		d.tmdbKey = ' key ';
		d.indexers = [{ definition: 'examplebay', settings: { base_url: 'http://examplebay.invalid/' } }];
		const input = toSetupInput(d);
		expect(input).toMatchObject({
			admin_password: 'correct horse',
			tmdb_api_key: 'key',
			downloads: { incomplete_dir: '/i', complete_dir: '/c', import_mode: 'hardlink' },
			movie_root: '/m',
			tv_root: undefined,
			vpn: undefined,
			indexers: [{ definition: 'examplebay', settings: { base_url: 'http://examplebay.invalid/' } }],
			create_missing: true
		});
		expect(input.downloads).not.toHaveProperty('network');

		d.vpnProvider = 'wireguard';
		d.vpnValues = { config: '[Interface]', note: '' };
		expect(toSetupInput(d).vpn).toEqual({ provider: 'wireguard', values: { config: '[Interface]' } });
	});
});
