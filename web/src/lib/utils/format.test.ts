import { describe, expect, it } from 'vitest';
import { episodeCode, formatBytes, formatETA, formatRate, formatRuntime, percent, timeAgo } from './format';

describe('format', () => {
	it('formats bytes with binary units', () => {
		expect(formatBytes(0)).toBe('0 B');
		expect(formatBytes(1023)).toBe('1023 B');
		expect(formatBytes(1536)).toBe('1.5 KiB');
		expect(formatBytes(5 * 1024 ** 3)).toBe('5.0 GiB');
	});
	it('computes bounded percentages', () => {
		expect(percent(1, 3)).toBe(33.3);
		expect(percent(5, 0)).toBe(0);
		expect(percent(9, 3)).toBe(100);
	});
	it('describes relative times', () => {
		const now = new Date('2025-01-02T12:00:00Z');
		expect(timeAgo('2025-01-02T11:59:30Z', now)).toBe('just now');
		expect(timeAgo('2025-01-02T10:00:00Z', now)).toBe('2 hours ago');
		expect(timeAgo('2025-01-01T12:00:00Z', now)).toBe('1 day ago');
		expect(timeAgo(undefined, now)).toBe('never');
		expect(timeAgo('0001-01-01T00:00:00Z', now)).toBe('unknown');
	});
	it('pads episode codes', () => {
		expect(episodeCode(1, 2)).toBe('S01E02');
		expect(episodeCode(12, 105)).toBe('S12E105');
	});
});

describe('transfer formatting', () => {
	it('formats rates', () => {
		expect(formatRate(0)).toBe('0 B/s');
		expect(formatRate(3.25 * 1024 * 1024)).toBe('3.3 MiB/s');
	});

	it('formats time left with its two largest units', () => {
		expect(formatETA(0)).toBe('—');
		expect(formatETA(Number.NaN)).toBe('—');
		expect(formatETA(42)).toBe('42s');
		expect(formatETA(245)).toBe('4m 05s');
		expect(formatETA(4_800)).toBe('1h 20m');
		expect(formatETA(2 * 86_400 + 3 * 3_600 + 59)).toBe('2d 3h');
	});
});

describe('formatRuntime', () => {
	it('shows hours and minutes, and nothing when unknown', () => {
		expect(formatRuntime(118)).toBe('1h 58m');
		expect(formatRuntime(47)).toBe('47m');
		expect(formatRuntime(120)).toBe('2h');
		expect(formatRuntime(0)).toBe('');
		expect(formatRuntime(undefined)).toBe('');
	});
});
