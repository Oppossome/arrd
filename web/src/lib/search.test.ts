import { describe, expect, it } from 'vitest';
import { wordMatcher } from './search';

describe('wordMatcher', () => {
	const site = ['Example Bay', 'examplebay', 'A fictional tracker for movies and TV. (public site)'];

	it('matches every word in any order and field', () => {
		expect(wordMatcher('public movie')(site)).toBe(true);
		expect(wordMatcher('MOVIE   example')(site)).toBe(true);
	});
	it('rejects when any word is missing', () => {
		expect(wordMatcher('public music')(site)).toBe(false);
		expect(wordMatcher('private')(site)).toBe(false);
	});
	it('matches everything for an empty query', () => {
		expect(wordMatcher('  ')(site)).toBe(true);
	});
	it('skips missing texts', () => {
		expect(wordMatcher('bay')(['Example Bay', undefined])).toBe(true);
	});
});
