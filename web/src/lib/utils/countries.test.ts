import { describe, expect, it } from 'vitest';
import { countryName, flagEmoji } from './countries';

describe('countries', () => {
	it('builds flags from ISO codes', () => {
		expect(flagEmoji('de')).toBe('🇩🇪');
		expect(flagEmoji('NZ')).toBe('🇳🇿');
		expect(flagEmoji('')).toBe('');
		expect(flagEmoji('XYZ')).toBe('');
	});

	it('names countries', () => {
		expect(countryName('')).toBe('Unknown location');
		expect(countryName('de')).toBe(new Intl.DisplayNames(undefined, { type: 'region' }).of('DE'));
	});
});
