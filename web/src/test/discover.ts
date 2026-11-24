// TMDB lookup results for discover tests (fictional titles).
import type { LookupMovie, LookupSeries } from '#lib/api/types.js';

export const lookupMovie = (tmdb_id: number, title: string, library_id?: number): LookupMovie => ({
	tmdb_id,
	title,
	original_title: title,
	year: 2031,
	overview: '',
	...(library_id ? { library_id } : {})
});

export const lookupSeries = (tmdb_id: number, title: string, library_id?: number): LookupSeries => ({
	tmdb_id,
	title,
	original_title: title,
	year: 2028,
	overview: '',
	...(library_id ? { library_id } : {})
});
