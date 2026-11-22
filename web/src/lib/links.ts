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

/** Browsing TMDB's catalogue, by genre (every genre when absent) and order. */
export function browseHref(kind: MediaKind, opts: { genre?: number; sort?: DiscoverSort } = {}): string {
	const params = new URLSearchParams();
	if (opts.genre !== undefined) params.set('genre', String(opts.genre));
	if (opts.sort !== undefined) params.set('sort', opts.sort);
	const query = params.toString();
	return `/${segment(kind)}/browse${query ? `?${query}` : ''}`;
}
