// The app's page URLs, built in one place.

import type { DiscoverSort } from './types';

export type MediaKind = 'movies' | 'series';

/** Where the app opens: popular movies. */
export const HOME = '/movies';

/** Everything in the library (downloaded or monitored). */
export const LIBRARY = '/library';

const segment = (kind: MediaKind) => (kind === 'movies' ? 'movies' : 'shows');

/** The discover page of a TMDB title: its torrents. */
export const discoverHref = (kind: MediaKind, tmdbId: number) => `/${segment(kind)}/${tmdbId}`;

/** A title's page in the library. */
export const libraryHref = (kind: MediaKind, id: number) => `${LIBRARY}/${segment(kind)}/${id}`;

/**
 * A discover page, searching or browsing TMDB's catalogue: by title, genres
 * (any of them), original language and order. Each is left out when absent,
 * and with none the page shows its curated lists.
 */
export function browseHref(
	kind: MediaKind,
	opts: { title?: string; genres?: number[]; language?: string; sort?: DiscoverSort } = {}
): string {
	const params = new URLSearchParams();
	if (opts.title !== undefined) params.set('q', opts.title);
	for (const genre of opts.genres ?? []) params.append('genre', String(genre));
	if (opts.language !== undefined) params.set('lang', opts.language);
	if (opts.sort !== undefined) params.set('sort', opts.sort);
	const query = params.toString();
	return `/${segment(kind)}${query ? `?${query}` : ''}`;
}
