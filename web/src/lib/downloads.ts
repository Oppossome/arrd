// What each library item's downloads add up to, for poster overlays and
// filters. Derived from the queue, so it needs no extra request.

import { percent } from './format';
import type { Download } from './types';

/** An item's download state: one or more running downloads, or a failure. */
export type ItemDownload =
	| {
			kind: 'active';
			/** 0–100 over all the item's running downloads. */
			percent: number;
			/** Every running download is paused. */
			paused: boolean;
			/** Every running download is finished and waiting to be imported. */
			importing: boolean;
	  }
	| { kind: 'failed' };

const isActive = (d: Download) => d.status === 'downloading' || d.status === 'completed';

/**
 * Groups downloads by the movie or series they are for. Running downloads
 * win; otherwise an item counts as failed only when its newest download
 * failed (a later success or removal clears it).
 */
export function itemDownloads(rows: Download[]): {
	movies: Map<number, ItemDownload>;
	series: Map<number, ItemDownload>;
} {
	const byItem = { movies: new Map<number, Download[]>(), series: new Map<number, Download[]>() };
	for (const d of rows) {
		const [map, id] = d.movie_id ? [byItem.movies, d.movie_id] : d.series_id ? [byItem.series, d.series_id] : [];
		if (!map || id === undefined) continue;
		map.set(id, [...(map.get(id) ?? []), d]);
	}
	const summarize = (m: Map<number, Download[]>) => {
		const out = new Map<number, ItemDownload>();
		for (const [id, ds] of m) {
			const state = summarizeItem(ds);
			if (state) out.set(id, state);
		}
		return out;
	};
	return { movies: summarize(byItem.movies), series: summarize(byItem.series) };
}

function summarizeItem(ds: Download[]): ItemDownload | undefined {
	const active = ds.filter(isActive);
	if (active.length > 0) {
		let done = 0;
		let total = 0;
		for (const d of active) {
			// Finished-but-unimported data counts as complete even without
			// live progress (e.g. after a restart).
			const t = d.progress?.bytes_total || d.size;
			total += t;
			done += d.status === 'completed' ? t : (d.progress?.bytes_completed ?? 0);
		}
		return {
			kind: 'active',
			percent: percent(done, total),
			paused: active.every((d) => d.paused),
			importing: active.every((d) => d.status === 'completed')
		};
	}
	const newest = ds.reduce((a, b) => (b.added_at > a.added_at ? b : a));
	return newest.status === 'failed' ? { kind: 'failed' } : undefined;
}

/** The queue rows the "Active now" shelf shows: running downloads, newest first. */
export function activeDownloads(rows: Download[]): Download[] {
	return rows.filter(isActive).sort((a, b) => b.added_at.localeCompare(a.added_at));
}
