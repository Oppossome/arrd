// TanStack Svelte Query setup: the client, query keys, query option
// builders and mutation option factories. Organisms call these through the
// `use*` hooks at the bottom; nothing else talks to the API directly.

import {
	MutationCache,
	QueryCache,
	QueryClient,
	createQueries,
	createMutation,
	useQueryClient,
	type CreateMutationOptions,
	type CreateMutationResult,
	type QueryKey
} from '@tanstack/svelte-query';
import { ApiError, api, filterQuery } from './client';
import type * as T from './types';
import type { DiscoverList } from '../domain/discover';
import type { MediaKind } from '../nav/links';
import type { ReleaseSource } from '../domain/releases';
import { optionFields } from '../domain/settingsForm';

// ---------------------------------------------------------------- client

/** No retries for client errors: repeating them cannot succeed. */
export function shouldRetry(failureCount: number, error: unknown): boolean {
	if (error instanceof ApiError && [400, 401, 403, 404, 409, 429, 503].includes(error.status)) {
		return false;
	}
	return failureCount < 2;
}

export interface ClientHooks {
	/** Called on 401 from any query or mutation. */
	onUnauthorized: () => void;
	/** Called when the API says onboarding is required. */
	onSetupRequired: () => void;
	/** Called for mutation errors other than the two above. */
	onMutationError: (message: string) => void;
}

export function createAppQueryClient(hooks: ClientHooks): QueryClient {
	const handleAuth = (error: unknown): boolean => {
		if (!(error instanceof ApiError)) return false;
		if (error.status === 401) {
			hooks.onUnauthorized();
			return true;
		}
		if (error.setupRequired) {
			hooks.onSetupRequired();
			return true;
		}
		return false;
	};
	return new QueryClient({
		queryCache: new QueryCache({ onError: (error) => void handleAuth(error) }),
		mutationCache: new MutationCache({
			onError: (error) => {
				if (!handleAuth(error)) hooks.onMutationError(error instanceof Error ? error.message : String(error));
			}
		}),
		defaultOptions: {
			queries: {
				staleTime: 30_000,
				refetchOnWindowFocus: true,
				refetchIntervalInBackground: false,
				retry: shouldRetry
			},
			mutations: { retry: false }
		}
	});
}

// ---------------------------------------------------------------- keys

export interface QueueFilter {
	movieId?: number;
	seriesId?: number;
}

const NO_FILTER: T.TitleFilter = { genres: [] };
/** A title filter as query key parts: sorted genres, so their order does not matter. */
const filterKey = (f: T.TitleFilter) => [[...f.genres].sort((a, b) => a - b), f.language ?? null] as const;

export const keys = {
	session: ['session'] as const,
	status: ['status'] as const,
	setupStatus: ['setup', 'status'] as const,
	dirs: (path: string) => ['dirs', path] as const,
	vpnProviders: {
		list: ['vpnProviders'] as const,
		options: (provider: string, field: string) => ['vpnProviders', 'options', provider, field] as const
	},
	discover: {
		all: ['discover'] as const,
		/** Every curated list, browse and "like this" row (they mark titles in the library). */
		allLists: ['discover', 'list'] as const,
		allBrowse: ['discover', 'browse'] as const,
		allLike: ['discover', 'like'] as const,
		list: (kind: MediaKind, list: DiscoverList) => ['discover', 'list', kind, list] as const,
		genres: (kind: MediaKind) => ['discover', 'genres', kind] as const,
		languages: ['discover', 'languages'] as const,
		browse: (kind: MediaKind, filter: T.TitleFilter, sort: T.DiscoverSort) =>
			['discover', 'browse', kind, ...filterKey(filter), sort] as const,
		like: (kind: MediaKind, tmdbId: number) => ['discover', 'like', kind, tmdbId] as const,
		recommendations: (kind: 'movies' | 'series') => ['discover', 'recommendations', kind] as const,
		allRecommendations: ['discover', 'recommendations'] as const,
		/** Every movie or show detail (not their release searches). */
		movieDetails: ['discover', 'movie'] as const,
		showDetails: ['discover', 'show'] as const,
		movie: (tmdbId: number) => ['discover', 'movie', tmdbId] as const,
		show: (tmdbId: number) => ['discover', 'show', tmdbId] as const,
		movieReleases: (tmdbId: number) => ['discover', 'movieReleases', tmdbId] as const,
		seriesReleases: (tmdbId: number) => ['discover', 'seriesReleases', tmdbId] as const,
		seasonReleases: (tmdbId: number, season: number) => ['discover', 'seasonReleases', tmdbId, season] as const,
		episodeReleases: (tmdbId: number, season: number, episode: number) =>
			['discover', 'episodeReleases', tmdbId, season, episode] as const,
		season: (tmdbId: number, season: number) => ['discover', 'season', tmdbId, season] as const
	},
	movies: {
		all: ['movies'] as const,
		list: ['movies', 'list'] as const,
		detail: (id: number) => ['movies', 'detail', id] as const,
		releases: (id: number) => ['movies', 'releases', id] as const,
		lookup: (q: string, filter: T.TitleFilter = NO_FILTER) => ['movies', 'lookup', q, ...filterKey(filter)] as const
	},
	series: {
		all: ['series'] as const,
		list: ['series', 'list'] as const,
		detail: (id: number) => ['series', 'detail', id] as const,
		episodes: (id: number) => ['series', 'episodes', id] as const,
		releases: (id: number) => ['series', 'releases', id] as const,
		seasonReleases: (id: number, season: number) => ['series', 'seasonReleases', id, season] as const,
		lookup: (q: string, filter: T.TitleFilter = NO_FILTER) => ['series', 'lookup', q, ...filterKey(filter)] as const
	},
	episodes: {
		releases: (id: number) => ['episodes', 'releases', id] as const
	},
	queue: {
		all: ['queue'] as const,
		list: (f: QueueFilter = {}) => ['queue', f] as const,
		peers: (id: number) => ['queue', 'peers', id] as const,
		throughput: ['queue', 'throughput'] as const
	},
	settings: {
		all: ['settings'] as const,
		general: ['settings', 'general'] as const,
		network: ['settings', 'network'] as const,
		downloads: ['settings', 'downloads'] as const,
		quality: ['settings', 'quality'] as const,
		vpn: ['settings', 'vpn'] as const
	},
	rootFolders: {
		all: ['rootFolders'] as const,
		list: (mt: T.MediaType) => ['rootFolders', mt] as const
	},
	indexers: ['indexers'] as const,
	indexerDefinitions: ['indexerDefinitions'] as const,
	mediaServers: ['mediaServers'] as const,
	mediaServerDefinitions: ['mediaServerDefinitions'] as const
};

// ---------------------------------------------------------------- queries

/** Polling interval for the queue: fast while something is in flight. */
export function queueRefetchInterval(rows: T.Download[] | undefined): number {
	const active = rows?.some((d) => d.status === 'downloading' || d.status === 'completed');
	return active ? 2_000 : 15_000;
}

/** Polling interval for transfer rates: every 2s while anything is active. */
export function throughputRefetchInterval(t: T.Throughput | undefined): number {
	return t && t.active > 0 ? 2_000 : 15_000;
}

export type { ReleaseSource };

export function releaseKey(source: ReleaseSource): QueryKey {
	switch (source.kind) {
		case 'movie':
			return keys.movies.releases(source.movieId);
		case 'series':
			return keys.series.releases(source.seriesId);
		case 'season':
			return keys.series.seasonReleases(source.seriesId, source.season);
		case 'episode':
			return keys.episodes.releases(source.episodeId);
		case 'discoverMovie':
			return keys.discover.movieReleases(source.tmdbId);
		case 'discoverSeries':
			return keys.discover.seriesReleases(source.tmdbId);
		case 'discoverSeason':
			return keys.discover.seasonReleases(source.tmdbId, source.season);
		case 'discoverEpisode':
			return keys.discover.episodeReleases(source.tmdbId, source.season, source.episode);
	}
}

/** Searches for source; text, when given, replaces arrd's own query. */
function searchReleases(source: ReleaseSource, text?: string): Promise<T.SearchResult> {
	switch (source.kind) {
		case 'movie':
			return api.movies.releases(source.movieId, text);
		case 'series':
			return api.series.releases(source.seriesId, text);
		case 'season':
			return api.series.seasonReleases(source.seriesId, source.season, text);
		case 'episode':
			return api.episodes.releases(source.episodeId, text);
		case 'discoverMovie':
			return api.discover.movieReleases(source.tmdbId, text);
		case 'discoverSeries':
			return api.discover.seriesReleases(source.tmdbId, text);
		case 'discoverSeason':
			return api.discover.seasonReleases(source.tmdbId, source.season, text);
		case 'discoverEpisode':
			return api.discover.episodeReleases(source.tmdbId, source.season, source.episode, text);
	}
}

const SEARCH_CACHE ={ staleTime: 5 * 60_000, gcTime: 10 * 60_000, enabled: false } as const;

export const queries = {
	session: () => ({ queryKey: keys.session, queryFn: api.session, staleTime: 0 }),
	status: () => ({ queryKey: keys.status, queryFn: api.status, refetchInterval: 15_000 }),
	setupStatus: () => ({ queryKey: keys.setupStatus, queryFn: api.setup.status, staleTime: Infinity }),
	dirs: (path: string) => ({ queryKey: keys.dirs(path), queryFn: () => api.listDirs(path), staleTime: 10_000 }),
	// Provider forms change only with arrd itself; option lists (e.g. PIA
	// regions) change slowly.
	vpnProviders: () => ({ queryKey: keys.vpnProviders.list, queryFn: api.vpnProviders.list, staleTime: Infinity }),
	vpnOptions: (provider: string, field: string) => ({
		queryKey: keys.vpnProviders.options(provider, field),
		queryFn: () => api.vpnProviders.options(provider, field),
		staleTime: 10 * 60_000
	}),

	movies: () => ({ queryKey: keys.movies.list, queryFn: api.movies.list }),
	movie: (id: number) => ({ queryKey: keys.movies.detail(id), queryFn: () => api.movies.get(id) }),
	movieReleases: (id: number) => ({
		queryKey: keys.movies.releases(id),
		queryFn: () => api.movies.releases(id),
		...SEARCH_CACHE
	}),
	movieLookup: (q: string) => ({
		queryKey: keys.movies.lookup(q),
		queryFn: () => api.movies.lookup(q),
		enabled: q.trim().length >= 2,
		staleTime: 5 * 60_000
	}),

	// One builder for both kinds of TMDB lookup (same keys as above), narrowed by
	// a filter. The search bar submits whole titles, so any length goes.
	lookup: (kind: 'movies' | 'series', q: string, filter: T.TitleFilter = NO_FILTER) => ({
		queryKey: (kind === 'movies' ? keys.movies.lookup(q, filter) : keys.series.lookup(q, filter)) as QueryKey,
		queryFn: (): Promise<T.LookupMovie[]> =>
			kind === 'movies' ? api.movies.lookup(q, undefined, filter) : api.series.lookup(q, undefined, filter),
		enabled: q.trim() !== '',
		staleTime: 5 * 60_000
	}),

	seriesList: () => ({ queryKey: keys.series.list, queryFn: api.series.list }),
	series: (id: number) => ({ queryKey: keys.series.detail(id), queryFn: () => api.series.get(id) }),
	episodes: (id: number) => ({ queryKey: keys.series.episodes(id), queryFn: () => api.series.episodes(id) }),
	seasonReleases: (id: number, season: number) => ({
		queryKey: keys.series.seasonReleases(id, season),
		queryFn: () => api.series.seasonReleases(id, season),
		...SEARCH_CACHE
	}),
	episodeReleases: (id: number) => ({
		queryKey: keys.episodes.releases(id),
		queryFn: () => api.episodes.releases(id),
		...SEARCH_CACHE
	}),
	seriesLookup: (q: string) => ({
		queryKey: keys.series.lookup(q),
		queryFn: () => api.series.lookup(q),
		enabled: q.trim().length >= 2,
		staleTime: 5 * 60_000
	}),

	// Manual release search for any target; started explicitly with refetch().
	// text, when given, replaces arrd's own query and is cached on its own.
	releases: (source: ReleaseSource, text = '') => ({
		queryKey: [...releaseKey(source), text] as QueryKey,
		queryFn: (): Promise<T.SearchResult> => searchReleases(source, text || undefined),
		...SEARCH_CACHE
	}),

	// TMDB's lists change daily at most; its genres practically never.
	discoverList: (kind: MediaKind, list: DiscoverList) => ({
		queryKey: keys.discover.list(kind, list) as QueryKey,
		queryFn: (): Promise<(T.LookupMovie | T.LookupSeries)[]> =>
			kind === 'movies'
				? api.discover.movieList(list as T.MovieList)
				: api.discover.seriesList(list as T.SeriesList),
		staleTime: 30 * 60_000
	}),
	genres: (kind: MediaKind) => ({
		queryKey: keys.discover.genres(kind) as QueryKey,
		queryFn: (): Promise<T.Genre[]> => (kind === 'movies' ? api.discover.movieGenres() : api.discover.seriesGenres()),
		staleTime: Infinity
	}),
	languages: () => ({ queryKey: keys.discover.languages, queryFn: api.discover.languages, staleTime: Infinity }),
	/** For createInfiniteQuery: one TMDB page per page, until total_pages. */
	browse: (kind: MediaKind, filter: T.TitleFilter, sort: T.DiscoverSort) => ({
		queryKey: keys.discover.browse(kind, filter, sort) as QueryKey,
		queryFn: ({ pageParam }: { pageParam: number }): Promise<T.MovieBrowsePage | T.SeriesBrowsePage> => {
			const query = { ...filterQuery(filter), sort, page: pageParam };
			return kind === 'movies' ? api.discover.browseMovies(query) : api.discover.browseSeries(query);
		},
		initialPageParam: 1,
		getNextPageParam: (last: T.MovieBrowsePage | T.SeriesBrowsePage) =>
			last.page < last.total_pages ? last.page + 1 : undefined,
		staleTime: 30 * 60_000
	}),
	like: (kind: MediaKind, tmdbId: number) => ({
		queryKey: keys.discover.like(kind, tmdbId) as QueryKey,
		queryFn: (): Promise<(T.LookupMovie | T.LookupSeries)[]> =>
			kind === 'movies' ? api.discover.moviesLike(tmdbId) : api.discover.seriesLike(tmdbId),
		staleTime: 30 * 60_000
	}),
	// One builder for both kinds of "because you added" rows. Refetched when the
	// library changes (see libraryMarks), since rows are seeded by it.
	recommendations: (kind: 'movies' | 'series') => ({
		queryKey: keys.discover.recommendations(kind) as QueryKey,
		queryFn: (): Promise<(T.MovieRecommendations | T.SeriesRecommendations)[]> =>
			kind === 'movies' ? api.discover.movieRecommendations() : api.discover.seriesRecommendations(),
		staleTime: 30 * 60_000
	}),
	discoverMovie: (tmdbId: number) => ({ queryKey: keys.discover.movie(tmdbId), queryFn: () => api.discover.movie(tmdbId) }),
	discoverShow: (tmdbId: number) => ({ queryKey: keys.discover.show(tmdbId), queryFn: () => api.discover.show(tmdbId) }),
	// TMDB episode lists change rarely; a season's are fetched when its tab opens.
	discoverSeason: (tmdbId: number, season: number) => ({
		queryKey: keys.discover.season(tmdbId, season),
		queryFn: () => api.discover.season(tmdbId, season),
		staleTime: 30 * 60_000
	}),

	queue: (f: QueueFilter = {}) => ({
		queryKey: keys.queue.list(f),
		queryFn: () => api.queue.list(f),
		refetchInterval: (q: { state: { data?: T.Download[] } }) => queueRefetchInterval(q.state.data)
	}),
	// Live views: polled while shown, never served stale.
	downloadPeers: (id: number) => ({
		queryKey: keys.queue.peers(id),
		queryFn: () => api.queue.peers(id),
		staleTime: 0,
		refetchInterval: 2_000
	}),
	throughput: () => ({
		queryKey: keys.queue.throughput,
		queryFn: api.queue.throughput,
		staleTime: 0,
		refetchInterval: (q: { state: { data?: T.Throughput } }) => throughputRefetchInterval(q.state.data)
	}),

	generalSettings: () => ({ queryKey: keys.settings.general, queryFn: api.settings.general }),
	networkSettings: () => ({ queryKey: keys.settings.network, queryFn: api.settings.network }),
	downloadSettings: () => ({ queryKey: keys.settings.downloads, queryFn: api.settings.downloads }),
	qualitySettings: () => ({ queryKey: keys.settings.quality, queryFn: api.settings.quality }),
	vpnSettings: (poll: boolean) => ({
		queryKey: keys.settings.vpn,
		queryFn: api.settings.vpn,
		refetchInterval: poll ? 5_000 : (false as const)
	}),
	rootFolders: (mt: T.MediaType) => ({ queryKey: keys.rootFolders.list(mt), queryFn: () => api.rootFolders.list(mt) }),
	indexers: () => ({ queryKey: keys.indexers, queryFn: api.indexers.list }),
	// Definitions change only when their files are reloaded, uploaded or deleted.
	indexerDefinitions: () => ({ queryKey: keys.indexerDefinitions, queryFn: api.indexers.definitions, staleTime: Infinity }),
	mediaServers: () => ({ queryKey: keys.mediaServers, queryFn: api.mediaServers.list }),
	// The server types are built into arrd.
	mediaServerDefinitions: () => ({ queryKey: keys.mediaServerDefinitions, queryFn: api.mediaServers.definitions, staleTime: Infinity })
};

// ---------------------------------------------------------------- mutations

/** Queries that mark or leave out titles in the library: refreshed when it changes. */
const libraryMarks: QueryKey[] = [
	keys.discover.allLists,
	keys.discover.allBrowse,
	keys.discover.allLike,
	keys.discover.allRecommendations,
	keys.discover.movieDetails,
	keys.discover.showDetails,
	['movies', 'lookup'],
	['series', 'lookup']
];

const invalidate = (qc: QueryClient, ...queryKeys: QueryKey[]) =>
	Promise.all(queryKeys.map((queryKey) => qc.invalidateQueries({ queryKey })));

/**
 * Mutation option factories. Each one names exactly which cached data it
 * makes stale, so screens update without manual refetching.
 */
export const mutations = {
	login: (qc: QueryClient) => ({
		mutationFn: (password: string) => api.login(password),
		onSuccess: () => qc.resetQueries()
	}),
	logout: (qc: QueryClient) => ({
		mutationFn: () => api.logout(),
		onSuccess: () => qc.clear()
	}),
	validateTMDB: () => ({ mutationFn: (key: string) => api.setup.validateTMDB(key) }),
	validatePaths: () => ({ mutationFn: (body: Parameters<typeof api.setup.validatePaths>[0]) => api.setup.validatePaths(body) }),
	validateVPN: () => ({ mutationFn: (input: T.VPNInput) => api.setup.validateVPN(input) }),
	completeSetup: (qc: QueryClient) => ({
		mutationFn: (input: T.SetupInput) => api.setup.complete(input),
		onSuccess: () => invalidate(qc, keys.session, keys.setupStatus)
	}),

	addMovie: (qc: QueryClient) => ({
		mutationFn: (body: Parameters<typeof api.movies.add>[0]) => api.movies.add(body),
		onSuccess: () => invalidate(qc, keys.movies.list, keys.queue.all, ...libraryMarks)
	}),
	deleteMovie: (qc: QueryClient) => ({
		mutationFn: (id: number) => api.movies.remove(id),
		onSuccess: (_: void, id: number) => {
			qc.removeQueries({ queryKey: keys.movies.detail(id) });
			return invalidate(qc, keys.movies.list, ...libraryMarks);
		}
	}),
	searchMovie: (qc: QueryClient) => ({
		mutationFn: (id: number) => api.movies.search(id),
		onSuccess: (_: T.AutoSearchResult, id: number) => invalidate(qc, keys.queue.all, keys.movies.detail(id), keys.movies.list)
	}),
	grabMovie: (qc: QueryClient) => ({
		mutationFn: (v: { id: number; release: T.ReleaseResult }) => api.movies.grab(v.id, v.release),
		onSuccess: (_: T.Download, v: { id: number }) => invalidate(qc, keys.queue.all, keys.movies.detail(v.id), keys.movies.list)
	}),
	setMovieMonitored: (qc: QueryClient) => ({
		mutationFn: (v: { id: number; monitored: boolean }) => api.movies.update(v.id, { monitored: v.monitored }),
		onMutate: async (v: { id: number; monitored: boolean }) => {
			await qc.cancelQueries({ queryKey: keys.movies.detail(v.id) });
			const previous = qc.getQueryData<T.Movie>(keys.movies.detail(v.id));
			if (previous) qc.setQueryData<T.Movie>(keys.movies.detail(v.id), { ...previous, monitored: v.monitored });
			return { previous };
		},
		onError: (_e: unknown, v: { id: number }, ctx?: { previous?: T.Movie }) => {
			if (ctx?.previous) qc.setQueryData(keys.movies.detail(v.id), ctx.previous);
		},
		onSettled: (_d: unknown, _e: unknown, v: { id: number }) => invalidate(qc, keys.movies.detail(v.id), keys.movies.list)
	}),

	setSeriesMonitored: (qc: QueryClient) => ({
		mutationFn: (v: { id: number; monitored: boolean }) => api.series.update(v.id, { monitored: v.monitored }),
		onMutate: async (v: { id: number; monitored: boolean }) => {
			await qc.cancelQueries({ queryKey: keys.series.detail(v.id) });
			const previous = qc.getQueryData<T.Series>(keys.series.detail(v.id));
			if (previous) qc.setQueryData<T.Series>(keys.series.detail(v.id), { ...previous, monitored: v.monitored });
			return { previous };
		},
		onError: (_e: unknown, v: { id: number }, ctx?: { previous?: T.Series }) => {
			if (ctx?.previous) qc.setQueryData(keys.series.detail(v.id), ctx.previous);
		},
		onSettled: (_d: unknown, _e: unknown, v: { id: number }) => invalidate(qc, keys.series.detail(v.id), keys.series.list)
	}),
	addSeries: (qc: QueryClient) => ({
		mutationFn: (body: Parameters<typeof api.series.add>[0]) => api.series.add(body),
		onSuccess: () => invalidate(qc, keys.series.list, keys.queue.all, ...libraryMarks)
	}),
	deleteSeries: (qc: QueryClient) => ({
		mutationFn: (id: number) => api.series.remove(id),
		onSuccess: (_: void, id: number) => {
			qc.removeQueries({ queryKey: keys.series.detail(id) });
			qc.removeQueries({ queryKey: keys.series.episodes(id) });
			return invalidate(qc, keys.series.list, ...libraryMarks);
		}
	}),
	refreshSeries: (qc: QueryClient) => ({
		mutationFn: (id: number) => api.series.refresh(id),
		onSuccess: (_: T.Series, id: number) => invalidate(qc, keys.series.detail(id), keys.series.episodes(id), keys.series.list)
	}),
	searchSeriesWanted: (qc: QueryClient) => ({
		mutationFn: (id: number) => api.series.searchWanted(id),
		onSuccess: (_: void, id: number) => invalidate(qc, keys.queue.all, keys.series.episodes(id))
	}),
	searchSeason: (qc: QueryClient) => ({
		mutationFn: (v: { id: number; season: number }) => api.series.searchSeason(v.id, v.season),
		onSuccess: (_: T.AutoSearchResult[], v: { id: number }) => invalidate(qc, keys.queue.all, keys.series.episodes(v.id))
	}),
	searchEpisode: (qc: QueryClient) => ({
		mutationFn: (v: { episodeId: number; seriesId: number }) => api.episodes.search(v.episodeId),
		onSuccess: (_: T.AutoSearchResult, v: { seriesId: number }) => invalidate(qc, keys.queue.all, keys.series.episodes(v.seriesId))
	}),
	// Grabbing a discovered title may add it to the library.
	grabDiscoverMovie: (qc: QueryClient) => ({
		mutationFn: (v: { tmdbId: number; release: T.ReleaseResult }) => api.discover.grabMovie(v.tmdbId, v.release),
		onSuccess: () => invalidate(qc, keys.queue.all, keys.movies.list, ...libraryMarks)
	}),
	grabDiscoverSeries: (qc: QueryClient) => ({
		mutationFn: (v: { tmdbId: number; release: T.ReleaseResult }) => api.discover.grabSeries(v.tmdbId, v.release),
		// The series may already be in the library: its episodes change too.
		onSuccess: () =>
			invalidate(qc, keys.queue.all, keys.series.list, ['series', 'episodes'], ['series', 'detail'], ...libraryMarks)
	}),
	grabSeries: (qc: QueryClient) => ({
		mutationFn: (v: { id: number; release: T.ReleaseResult }) => api.series.grab(v.id, v.release),
		onSuccess: (_: T.Download, v: { id: number }) => invalidate(qc, keys.queue.all, keys.series.episodes(v.id), keys.series.detail(v.id))
	}),
	setEpisodeMonitored: (qc: QueryClient) => ({
		mutationFn: (v: { seriesId: number; episodeId: number; monitored: boolean }) =>
			api.episodes.setMonitored(v.episodeId, v.monitored),
		onMutate: async (v: { seriesId: number; episodeId: number; monitored: boolean }) => {
			const key = keys.series.episodes(v.seriesId);
			await qc.cancelQueries({ queryKey: key });
			const previous = qc.getQueryData<T.Episode[]>(key);
			if (previous) {
				qc.setQueryData<T.Episode[]>(key, previous.map((e) => (e.id === v.episodeId ? { ...e, monitored: v.monitored } : e)));
			}
			return { previous };
		},
		onError: (_e: unknown, v: { seriesId: number }, ctx?: { previous?: T.Episode[] }) => {
			if (ctx?.previous) qc.setQueryData(keys.series.episodes(v.seriesId), ctx.previous);
		},
		onSettled: (_d: unknown, _e: unknown, v: { seriesId: number }) =>
			invalidate(qc, keys.series.episodes(v.seriesId), keys.series.detail(v.seriesId))
	}),
	setSeasonMonitored: (qc: QueryClient) => ({
		mutationFn: (v: { seriesId: number; season: number; monitored: boolean }) =>
			api.series.setSeasonMonitored(v.seriesId, v.season, v.monitored),
		onMutate: async (v: { seriesId: number; season: number; monitored: boolean }) => {
			const key = keys.series.episodes(v.seriesId);
			await qc.cancelQueries({ queryKey: key });
			const previous = qc.getQueryData<T.Episode[]>(key);
			if (previous) {
				qc.setQueryData<T.Episode[]>(key, previous.map((e) => (e.season === v.season ? { ...e, monitored: v.monitored } : e)));
			}
			return { previous };
		},
		onError: (_e: unknown, v: { seriesId: number }, ctx?: { previous?: T.Episode[] }) => {
			if (ctx?.previous) qc.setQueryData(keys.series.episodes(v.seriesId), ctx.previous);
		},
		onSettled: (_d: unknown, _e: unknown, v: { seriesId: number }) =>
			invalidate(qc, keys.series.episodes(v.seriesId), keys.series.detail(v.seriesId))
	}),

	removeDownload: (qc: QueryClient) => ({
		mutationFn: (v: { id: number; deleteData: boolean }) => api.queue.remove(v.id, v.deleteData),
		onSuccess: () => invalidate(qc, keys.queue.all, keys.movies.all, keys.series.all)
	}),
	pauseDownload: (qc: QueryClient) => ({
		mutationFn: (id: number) => api.queue.pause(id),
		onSuccess: () => invalidate(qc, keys.queue.all)
	}),
	resumeDownload: (qc: QueryClient) => ({
		mutationFn: (id: number) => api.queue.resume(id),
		onSuccess: () => invalidate(qc, keys.queue.all)
	}),
	rssSync: (qc: QueryClient) => ({
		mutationFn: () => api.rssSync(),
		onSuccess: () => invalidate(qc, keys.queue.all)
	}),

	updateGeneral: (qc: QueryClient) => ({
		mutationFn: (tmdbKey: string) => api.settings.updateGeneral(tmdbKey),
		onSuccess: () => invalidate(qc, keys.settings.general)
	}),
	updateNetwork: (qc: QueryClient) => ({
		mutationFn: (exposeLAN: boolean) => api.settings.updateNetwork(exposeLAN),
		onSuccess: () => invalidate(qc, keys.settings.network)
	}),
	regenerateAPIKey: (qc: QueryClient) => ({
		mutationFn: () => api.settings.regenerateAPIKey(),
		onSuccess: () => invalidate(qc, keys.settings.general)
	}),
	changePassword: () => ({
		mutationFn: (v: { current: string; next: string }) => api.settings.changePassword(v.current, v.next)
	}),
	updateDownloads: (qc: QueryClient) => ({
		mutationFn: (s: T.DownloadSettings) => api.settings.updateDownloads(s),
		onSuccess: () => invalidate(qc, keys.settings.downloads, keys.status, keys.settings.vpn)
	}),
	updateQuality: (qc: QueryClient) => ({
		mutationFn: (q: T.QualitySettings) => api.settings.updateQuality(q),
		onSuccess: () => invalidate(qc, keys.settings.quality)
	}),
	setVPN: (qc: QueryClient) => ({
		mutationFn: (input: T.VPNInput) => api.settings.setVPN(input),
		onSuccess: () => invalidate(qc, keys.settings.vpn, keys.status)
	}),
	removeVPN: (qc: QueryClient) => ({
		mutationFn: () => api.settings.removeVPN(),
		onSuccess: () => invalidate(qc, keys.settings.vpn, keys.status)
	}),
	testVPN: () => ({ mutationFn: (input?: T.VPNInput) => api.settings.testVPN(input) }),
	addRootFolder: (qc: QueryClient) => ({
		mutationFn: (v: { mediaType: T.MediaType; path: string }) => api.rootFolders.add(v.mediaType, v.path),
		onSuccess: () => invalidate(qc, keys.rootFolders.all)
	}),
	setDefaultRootFolder: (qc: QueryClient) => ({
		mutationFn: (id: number) => api.rootFolders.setDefault(id),
		onSuccess: () => invalidate(qc, keys.rootFolders.all)
	}),
	deleteRootFolder: (qc: QueryClient) => ({
		mutationFn: (id: number) => api.rootFolders.remove(id),
		onSuccess: () => invalidate(qc, keys.rootFolders.all)
	}),
	saveIndexer: (qc: QueryClient) => ({
		mutationFn: (v: { id?: number; input: T.IndexerInput }) =>
			v.id ? api.indexers.update(v.id, v.input) : api.indexers.create(v.input),
		onSuccess: () => invalidate(qc, keys.indexers)
	}),
	deleteIndexer: (qc: QueryClient) => ({
		mutationFn: (id: number) => api.indexers.remove(id),
		onSuccess: () => invalidate(qc, keys.indexers)
	}),
	testIndexer: (qc: QueryClient) => ({
		mutationFn: (id: number) => api.indexers.test(id),
		// A failed test is recorded on the indexer too, so refresh either way.
		onSettled: () => invalidate(qc, keys.indexers)
	}),
	reloadIndexerDefinitions: (qc: QueryClient) => ({
		mutationFn: () => api.indexers.reloadDefinitions(),
		onSuccess: () => invalidate(qc, keys.indexerDefinitions)
	}),
	uploadIndexerDefinitions: (qc: QueryClient) => ({
		mutationFn: (file: Blob) => api.indexers.uploadDefinitions(file),
		onSuccess: () => invalidate(qc, keys.indexerDefinitions)
	}),
	deleteIndexerDefinition: (qc: QueryClient) => ({
		mutationFn: (id: string) => api.indexers.removeDefinition(id),
		onSuccess: () => invalidate(qc, keys.indexerDefinitions)
	}),
	saveMediaServer: (qc: QueryClient) => ({
		mutationFn: (v: { id?: number; input: T.MediaServerInput }) =>
			v.id ? api.mediaServers.update(v.id, v.input) : api.mediaServers.create(v.input),
		onSuccess: () => invalidate(qc, keys.mediaServers)
	}),
	deleteMediaServer: (qc: QueryClient) => ({
		mutationFn: (id: number) => api.mediaServers.remove(id),
		onSuccess: () => invalidate(qc, keys.mediaServers)
	}),
	testMediaServer: (qc: QueryClient) => ({
		mutationFn: (id: number) => api.mediaServers.test(id),
		// A failed test is recorded on the server too, so refresh either way.
		onSettled: () => invalidate(qc, keys.mediaServers)
	})
};

type MutationFactories = typeof mutations;
type MutationName = keyof MutationFactories;
type FactoryOptions<K extends MutationName> = ReturnType<MutationFactories[K]>;
/** The data a named mutation resolves to. */
export type MutationData<K extends MutationName> = Awaited<ReturnType<FactoryOptions<K>['mutationFn']>>;
/** The variables a named mutation takes. */
export type MutationVars<K extends MutationName> = Parameters<FactoryOptions<K>['mutationFn']>[0];

/** Creates a mutation from one of the factories above, bound to the app's client. */
export function useMutation<K extends MutationName>(
	name: K
): CreateMutationResult<MutationData<K>, Error, MutationVars<K>, unknown> {
	const qc = useQueryClient();
	// Each factory's options are precisely typed; the map's union of them is
	// not assignable to one generic signature, hence the single cast here.
	const factory = mutations[name] as unknown as (
		qc: QueryClient
	) => CreateMutationOptions<MutationData<K>, Error, MutationVars<K>, unknown>;
	return createMutation(() => factory(qc));
}

/** Option lists for a VPN provider's select fields, by field key. */
export interface VPNOptionsState {
	options: Record<string, T.SettingOption[] | undefined>;
	/** Field keys still loading. */
	loading: string[];
	error: Error | null;
}

/**
 * Loads the choices for every select field of the current provider (e.g.
 * PIA's regions). Call during component initialisation.
 */
export function useVPNOptions(provider: () => T.VPNProvider | undefined): VPNOptionsState {
	return createQueries(() => {
		const p = provider();
		const fields = optionFields(p);
		return {
			queries: fields.map((f) => queries.vpnOptions(p?.id ?? '', f.key)),
			combine: (results: { data?: T.SettingOption[]; isPending: boolean; error: Error | null }[]): VPNOptionsState => {
				const out: VPNOptionsState = { options: {}, loading: [], error: null };
				results.forEach((r, i) => {
					const key = fields[i].key;
					out.options[key] = r.data;
					if (r.isPending) out.loading.push(key);
					out.error ??= r.error;
				});
				return out;
			}
		};
	});
}
