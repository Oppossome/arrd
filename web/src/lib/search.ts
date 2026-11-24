/**
 * Builds a matcher for a free-text filter: every word of the query must
 * appear somewhere in the texts, in any order and ignoring case. So
 * "public movie" matches a site whose description says "Movies and TV
 * (public site)". An empty query matches everything.
 */
export function wordMatcher(query: string): (texts: (string | undefined)[]) => boolean {
	const words = query.toLowerCase().split(/\s+/).filter(Boolean);
	return (texts) => {
		if (words.length === 0) return true;
		const haystack = texts.filter(Boolean).join(' ').toLowerCase();
		return words.every((w) => haystack.includes(w));
	};
}
