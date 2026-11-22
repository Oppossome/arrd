// Country display helpers for ISO 3166-1 alpha-2 codes.

const names =
	typeof Intl !== 'undefined' && 'DisplayNames' in Intl ? new Intl.DisplayNames(undefined, { type: 'region' }) : undefined;

/** The country's name in the user's language, the code if unknown, or "Unknown location" for "". */
export function countryName(code: string): string {
	if (!code) return 'Unknown location';
	try {
		return names?.of(code.toUpperCase()) ?? code;
	} catch {
		return code;
	}
}

/** The regional-indicator emoji flag for a code; "" if it is not two letters. */
export function flagEmoji(code: string): string {
	if (!/^[A-Za-z]{2}$/.test(code)) return '';
	return String.fromCodePoint(...[...code.toUpperCase()].map((c) => 0x1f1e6 + c.charCodeAt(0) - 65));
}
