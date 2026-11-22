// What the discover pages show: TMDB's curated lists in order, browse sort
// orders, and how a TMDB title becomes a poster card.

import { posterURL } from './api';
import { discoverHref, type MediaKind } from './links';
import type * as T from './types';

export type DiscoverList = T.MovieList | T.SeriesList;

export interface DiscoverRow {
	list: DiscoverList;
	title: string;
}

const movieRows: { list: T.MovieList; title: string }[] = [
	{ list: 'trending', title: 'Trending this week' },
	{ list: 'popular', title: 'Popular' },
	{ list: 'now_playing', title: 'In theaters' },
	{ list: 'upcoming', title: 'Coming soon' },
	{ list: 'top_rated', title: 'Top rated' }
];

const seriesRows: { list: T.SeriesList; title: string }[] = [
	{ list: 'trending', title: 'Trending this week' },
	{ list: 'popular', title: 'Popular' },
	{ list: 'airing_today', title: 'Airing today' },
	{ list: 'on_the_air', title: 'On the air' },
	{ list: 'top_rated', title: 'Top rated' }
];

/** The curated rows of a discover page, top to bottom. */
export const discoverRows = (kind: MediaKind): DiscoverRow[] => (kind === 'movies' ? movieRows : seriesRows);

export const DEFAULT_SORT: T.DiscoverSort = 'popular';

export const sortOptions: { value: T.DiscoverSort; label: string }[] = [
	{ value: 'popular', label: 'Most popular' },
	{ value: 'top_rated', label: 'Top rated' },
	{ value: 'newest', label: 'Newest' }
];

/** A browse's sort order from the URL; the default when missing or unknown. */
export function parseSort(value: string | null): T.DiscoverSort {
	return sortOptions.find((o) => o.value === value)?.value ?? DEFAULT_SORT;
}

/** A browse's genre id from the URL; undefined (every genre) unless a positive integer. */
export function parseGenre(value: string | null): number | undefined {
	if (value === null || !/^\d+$/.test(value)) return undefined;
	const id = Number(value);
	return id > 0 && Number.isSafeInteger(id) ? id : undefined;
}

/** PosterCard props for a TMDB title, linking to its discover page. */
export function posterCardProps(kind: MediaKind, item: T.LookupMovie | T.LookupSeries) {
	return {
		href: discoverHref(kind, item.tmdb_id),
		title: item.title,
		year: item.year,
		posterSrc: posterURL(item.poster_path, 'w342'),
		subtitle: item.library_id ? 'In library' : undefined
	};
}
