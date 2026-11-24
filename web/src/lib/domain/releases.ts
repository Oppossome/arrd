import type { ReleaseResult } from '../api/types';

/** What a manual release search is for. */
export type ReleaseSource =
	| { kind: 'movie'; movieId: number }
	| { kind: 'series'; seriesId: number }
	| { kind: 'season'; seriesId: number; season: number }
	| { kind: 'episode'; seriesId: number; episodeId: number }
	| { kind: 'discoverMovie'; tmdbId: number }
	| { kind: 'discoverSeries'; tmdbId: number }
	| { kind: 'discoverSeason'; tmdbId: number; season: number }
	| { kind: 'discoverEpisode'; tmdbId: number; season: number; episode: number };

/** A request to show the sources (releases) for something, with a heading for them. */
export interface SourcesRequest {
	title: string;
	source: ReleaseSource;
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
 * "Complete series", "S01-S03 pack", "S01 pack", "S01E02-E03", "S01E02" or
 * "" for a release's TV coverage.
 */
export function releaseCoverage(r: ReleaseResult): string {
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
