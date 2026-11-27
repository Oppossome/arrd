import type { ReleaseResult, ReleaseTarget } from '../api/types';

export type { ReleaseTarget };

/** A request to show the sources (releases) for a target, with a heading for them. */
export interface SourcesRequest {
	title: string;
	target: ReleaseTarget;
}

export type ReleaseSortKey = 'rank' | 'quality' | 'size' | 'seeders' | 'age';

/**
 * Sorts releases for display. "rank" keeps arrd's own order (accepted
 * first, then quality score, then seeders); other keys sort descending
 * except age, which shows the newest first.
 */
export function sortReleases(releases: ReleaseResult[], key: ReleaseSortKey): ReleaseResult[] {
	const out = [...releases];
	if (key === 'rank') return out;
	const val = (r: ReleaseResult): number => {
		switch (key) {
			case 'quality':
				return r.score;
			case 'size':
				return r.size;
			case 'seeders':
				return r.seeders;
			case 'age':
				return new Date(r.publish_date).getTime() || 0;
		}
	};
	return out.sort((a, b) => {
		if (a.accepted !== b.accepted) return a.accepted ? -1 : 1;
		return val(b) - val(a);
	});
}

/**
 * "Movie pack", "Complete series", "S01-S03 pack", "S01 pack",
 * "S01E02-E03", "S01E02" or "" for what a release covers.
 */
export function releaseCoverage(r: ReleaseResult): string {
	if (r.movie_pack) return 'Movie pack';
	if (r.complete_series) return 'Complete series';
	if (r.season === undefined) return '';
	const code = (n: number) => `S${String(n).padStart(2, '0')}`;
	const s = code(r.season);
	if (r.season_pack) return r.last_season !== undefined ? `${s}-${code(r.last_season)} pack` : `${s} pack`;
	const eps = r.episodes ?? [];
	if (eps.length === 0) return s;
	const e = (n: number) => `E${String(n).padStart(2, '0')}`;
	return eps.length === 1 ? s + e(eps[0]) : `${s}${e(eps[0])}-${e(eps[eps.length - 1])}`;
}

/**
 * Narrows search results on the client. An empty list places no limit on
 * its field.
 */
export interface ReleaseFilter {
	acceptedOnly: boolean;
	qualities: string[];
	indexers: string[];
}

export const emptyReleaseFilter = (): ReleaseFilter => ({ acceptedOnly: false, qualities: [], indexers: [] });

export function isFilterEmpty(f: ReleaseFilter): boolean {
	return !f.acceptedOnly && f.qualities.length === 0 && f.indexers.length === 0;
}

export function filterReleases(releases: ReleaseResult[], f: ReleaseFilter): ReleaseResult[] {
	return releases.filter(
		(r) =>
			(!f.acceptedOnly || r.accepted) &&
			(f.qualities.length === 0 || f.qualities.includes(r.quality)) &&
			(f.indexers.length === 0 || f.indexers.includes(r.indexer))
	);
}

/** A value to filter by and how many results have it. */
export interface Facet {
	value: string;
	count: number;
}

/**
 * The qualities (best first, by the highest score seen) and indexers (most
 * results first) in a set of results.
 */
export function releaseFacets(releases: ReleaseResult[]): { qualities: Facet[]; indexers: Facet[] } {
	const qualities = new Map<string, Facet & { best: number }>();
	const indexers = new Map<string, Facet>();
	for (const r of releases) {
		const q = qualities.get(r.quality) ?? { value: r.quality, count: 0, best: r.score };
		q.count++;
		q.best = Math.max(q.best, r.score);
		qualities.set(r.quality, q);
		const ix = indexers.get(r.indexer) ?? { value: r.indexer, count: 0 };
		ix.count++;
		indexers.set(r.indexer, ix);
	}
	const byCount = (a: Facet, b: Facet) => b.count - a.count || a.value.localeCompare(b.value);
	return {
		qualities: [...qualities.values()].sort((a, b) => b.best - a.best || byCount(a, b)).map(({ value, count }) => ({ value, count })),
		indexers: [...indexers.values()].sort(byCount)
	};
}

/** Adds value to list, or takes it out if it is there. */
export function toggleValue(list: string[], value: string): string[] {
	return list.includes(value) ? list.filter((v) => v !== value) : [...list, value];
}

/** Why a release was rejected, on one line. */
export function rejectionSummary(r: ReleaseResult): string {
	return (r.rejections ?? []).join(' · ');
}
