import { describe, expect, it } from 'vitest';
import { gateRedirect, safeNext } from './auth-gate';

const at = (path: string) => new URL(path, 'http://arrd.local');

describe('gateRedirect', () => {
	const notSetUp = { setup_complete: false, authenticated: false };
	const signedOut = { setup_complete: true, authenticated: false };
	const signedIn = { setup_complete: true, authenticated: true };

	it('sends everyone to setup until onboarding is done, keeping the token', () => {
		expect(gateRedirect(notSetUp, at('/movies'))).toBe('/setup');
		expect(gateRedirect(notSetUp, at('/?token=abc'))).toBe('/setup?token=abc');
		expect(gateRedirect(notSetUp, at('/setup?token=abc'))).toBeNull();
	});
	it('sends signed-out users to login with where they were going', () => {
		expect(gateRedirect(signedOut, at('/movies/5?x=1'))).toBe('/login?next=%2Fmovies%2F5%3Fx%3D1');
		expect(gateRedirect(signedOut, at('/'))).toBe('/login');
		expect(gateRedirect(signedOut, at('/setup'))).toBe('/login');
		expect(gateRedirect(signedOut, at('/login'))).toBeNull();
	});
	it('moves signed-in users off setup and login', () => {
		expect(gateRedirect(signedIn, at('/login?next=/series/2'))).toBe('/series/2');
		expect(gateRedirect(signedIn, at('/setup'))).toBe('/movies');
		expect(gateRedirect(signedIn, at('/'))).toBe('/movies');
		expect(gateRedirect(signedIn, at('/activity'))).toBeNull();
	});
	it('refuses off-site next targets', () => {
		expect(safeNext('//evil.example')).toBeNull();
		expect(safeNext('https://evil.example')).toBeNull();
		expect(safeNext('/\\evil')).toBeNull();
		expect(safeNext('/movies')).toBe('/movies');
	});
});
