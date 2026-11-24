// Typed client for arrd's JSON API. Paths, parameters and bodies are checked
// against tools/openapi.yaml (via the generated api.gen.ts).
// Requests use the session cookie (same-origin); during onboarding the
// setup token is sent instead.

import createClient, { type Client, type Middleware } from 'openapi-fetch';
import type { paths } from './api.gen';
import type * as T from './types';

/** An error response from the API. */
export class ApiError extends Error {
	readonly status: number;
	readonly field?: string;

	constructor(status: number, message: string, field?: string) {
		super(message);
		this.name = 'ApiError';
		this.status = status;
		this.field = field;
	}

	/** True for "onboarding has not been completed yet". */
	get setupRequired(): boolean {
		return this.status === 503 && this.message.startsWith('setup required');
	}
}

const SETUP_TOKEN_KEY = 'arrd.setupToken';

/** The setup token, kept in sessionStorage only while onboarding. */
export const setupToken = {
	get(): string | null {
		return typeof sessionStorage === 'undefined' ? null : sessionStorage.getItem(SETUP_TOKEN_KEY);
	},
	set(token: string): void {
		sessionStorage.setItem(SETUP_TOKEN_KEY, token);
	},
	clear(): void {
		sessionStorage.removeItem(SETUP_TOKEN_KEY);
	}
};

/** Builds an ApiError from a failed response. */
export async function toApiError(res: Response): Promise<ApiError> {
	let message = res.statusText || `HTTP ${res.status}`;
	let field: string | undefined;
	try {
		const body: unknown = await res.json();
		if (body && typeof body === 'object' && 'error' in body && typeof body.error === 'string') {
			message = body.error;
			if ('field' in body && typeof body.field === 'string') field = body.field;
		}
	} catch {
		// not JSON: keep the status text
	}
	return new ApiError(res.status, message, field);
}

// Only routes the wizard needs before onboarding accept the token.
const SETUP_PATHS = ['/api/v1/setup', '/api/v1/fs/', '/api/v1/vpn/', '/api/v1/indexers/definitions'];

const middleware: Middleware = {
	onRequest({ request, schemaPath }) {
		const token = setupToken.get();
		if (token && SETUP_PATHS.some((p) => schemaPath.startsWith(p))) {
			request.headers.set('X-Setup-Token', token);
		}
		return request;
	},
	async onResponse({ response }) {
		if (!response.ok) throw await toApiError(response);
		return undefined;
	}
};

let client: Client<paths> | undefined;

/** The API client, created on first use (the page origin is known by then). */
function http(): Client<paths> {
	if (!client) {
		client = createClient<paths>({
			baseUrl: location.origin,
			credentials: 'same-origin',
			headers: { Accept: 'application/json' },
			// Looked up per request so tests can stub the global fetch.
			fetch: (request) => globalThis.fetch(request)
		});
		client.use(middleware);
	}
	return client;
}

/** The body of a successful response (failures throw ApiError). */
async function body<D>(result: Promise<{ data?: D }>): Promise<D> {
	return (await result).data as D;
}

/** Waits for a successful response without a body. */
async function done(result: Promise<unknown>): Promise<void> {
	await result;
}

const id = (id: number) => ({ params: { path: { id } } });
const season = (id: number, season: number) => ({ params: { path: { id, season } } });
const tmdb = (tmdbId: number) => ({ params: { path: { tmdbId } } });
/** Path parameters plus a release search's own text, left out when blank. */
const searchText = <P extends object>(at: { params: { path: P } }, q?: string) => ({
	params: { ...at.params, query: { q: q?.trim() || undefined } }
});
/** The genre and lang query parameters of a title filter, left out when unset. */
export const filterQuery = (f?: T.TitleFilter) => ({
	genre: f?.genres.length ? f.genres : undefined,
	lang: f?.language
});

export const api = {
	session: () => body(http().GET('/api/v1/auth/session')),
	login: (password: string) => done(http().POST('/api/v1/auth/login', { body: { password } })),
	logout: () => done(http().POST('/api/v1/auth/logout')),
	status: () => body(http().GET('/api/v1/status')),

	setup: {
		status: () => body(http().GET('/api/v1/setup/status')),
		validatePaths: (input: T.PathsInput) => body(http().POST('/api/v1/setup/validate/paths', { body: input })),
		validateVPN: (input: T.VPNInput) => body(http().POST('/api/v1/setup/validate/vpn', { body: input })),
		validateTMDB: (tmdb_api_key: string) => body(http().POST('/api/v1/setup/validate/tmdb', { body: { tmdb_api_key } })),
		complete: (input: T.SetupInput) => body(http().POST('/api/v1/setup', { body: input }))
	},

	listDirs: (path: string) => body(http().GET('/api/v1/fs/dirs', { params: { query: { path: path || undefined } } })),

	vpnProviders: {
		list: () => body(http().GET('/api/v1/vpn/providers')),
		options: (provider: string, field: string) =>
			body(http().GET('/api/v1/vpn/providers/{id}/options/{field}', { params: { path: { id: provider, field } } }))
	},

	/** TMDB's catalogue, browsable, searchable and downloadable without adding titles first. */
	discover: {
		movieList: (list: T.MovieList) =>
			body(http().GET('/api/v1/discover/lists/movies/{list}', { params: { path: { list } } })),
		languages: () => body(http().GET('/api/v1/discover/languages')),
		movieGenres: () => body(http().GET('/api/v1/discover/movies/genres')),
		browseMovies: (query: T.BrowseQuery) => body(http().GET('/api/v1/discover/movies/browse', { params: { query } })),
		moviesLike: (tmdbId: number) => body(http().GET('/api/v1/discover/movies/{tmdbId}/recommendations', tmdb(tmdbId))),
		movieRecommendations: () => body(http().GET('/api/v1/discover/movies/recommendations')),
		movie: (tmdbId: number) => body(http().GET('/api/v1/discover/movies/{tmdbId}', tmdb(tmdbId))),
		movieReleases: (tmdbId: number, q?: string) =>
			body(http().GET('/api/v1/discover/movies/{tmdbId}/releases', searchText(tmdb(tmdbId), q))),
		grabMovie: (tmdbId: number, release: T.ReleaseResult) =>
			body(http().POST('/api/v1/discover/movies/{tmdbId}/grab', { ...tmdb(tmdbId), body: release })),
		seriesList: (list: T.SeriesList) =>
			body(http().GET('/api/v1/discover/lists/series/{list}', { params: { path: { list } } })),
		seriesGenres: () => body(http().GET('/api/v1/discover/series/genres')),
		browseSeries: (query: T.BrowseQuery) => body(http().GET('/api/v1/discover/series/browse', { params: { query } })),
		seriesLike: (tmdbId: number) => body(http().GET('/api/v1/discover/series/{tmdbId}/recommendations', tmdb(tmdbId))),
		seriesRecommendations: () => body(http().GET('/api/v1/discover/series/recommendations')),
		show: (tmdbId: number) => body(http().GET('/api/v1/discover/series/{tmdbId}', tmdb(tmdbId))),
		seriesReleases: (tmdbId: number, q?: string) =>
			body(http().GET('/api/v1/discover/series/{tmdbId}/releases', searchText(tmdb(tmdbId), q))),
		seasonReleases: (tmdbId: number, season: number, q?: string) =>
			body(
				http().GET(
					'/api/v1/discover/series/{tmdbId}/seasons/{season}/releases',
					searchText({ params: { path: { tmdbId, season } } }, q)
				)
			),
		season: (tmdbId: number, season: number) =>
			body(http().GET('/api/v1/discover/series/{tmdbId}/seasons/{season}', { params: { path: { tmdbId, season } } })),
		episodeReleases: (tmdbId: number, season: number, episode: number, q?: string) =>
			body(
				http().GET(
					'/api/v1/discover/series/{tmdbId}/seasons/{season}/episodes/{episode}/releases',
					searchText({ params: { path: { tmdbId, season, episode } } }, q)
				)
			),
		grabSeries: (tmdbId: number, release: T.ReleaseResult) =>
			body(http().POST('/api/v1/discover/series/{tmdbId}/grab', { ...tmdb(tmdbId), body: release }))
	},

	movies: {
		list: () => body(http().GET('/api/v1/movies')),
		get: (movieId: number) => body(http().GET('/api/v1/movies/{id}', id(movieId))),
		lookup: (q: string, year?: number, filter?: T.TitleFilter) =>
			body(http().GET('/api/v1/movies/lookup', { params: { query: { q: q || undefined, year, ...filterQuery(filter) } } })),
		add: (input: T.AddMovieRequest) => body(http().POST('/api/v1/movies', { body: input })),
		update: (movieId: number, input: T.UpdateMovieRequest) =>
			body(http().PUT('/api/v1/movies/{id}', { ...id(movieId), body: input })),
		remove: (movieId: number) => done(http().DELETE('/api/v1/movies/{id}', id(movieId))),
		releases: (movieId: number, q?: string) => body(http().GET('/api/v1/movies/{id}/releases', searchText(id(movieId), q))),
		search: (movieId: number) => body(http().POST('/api/v1/movies/{id}/search', id(movieId))),
		grab: (movieId: number, release: T.ReleaseResult) =>
			body(http().POST('/api/v1/movies/{id}/grab', { ...id(movieId), body: release }))
	},

	series: {
		list: () => body(http().GET('/api/v1/series')),
		get: (seriesId: number) => body(http().GET('/api/v1/series/{id}', id(seriesId))),
		episodes: (seriesId: number) => body(http().GET('/api/v1/series/{id}/episodes', id(seriesId))),
		lookup: (q: string, year?: number, filter?: T.TitleFilter) =>
			body(http().GET('/api/v1/series/lookup', { params: { query: { q: q || undefined, year, ...filterQuery(filter) } } })),
		add: (input: T.AddSeriesRequest) => body(http().POST('/api/v1/series', { body: input })),
		update: (seriesId: number, input: T.UpdateSeriesRequest) =>
			body(http().PUT('/api/v1/series/{id}', { ...id(seriesId), body: input })),
		remove: (seriesId: number) => done(http().DELETE('/api/v1/series/{id}', id(seriesId))),
		refresh: (seriesId: number) => body(http().POST('/api/v1/series/{id}/refresh', id(seriesId))),
		searchWanted: (seriesId: number) => done(http().POST('/api/v1/series/{id}/search', id(seriesId))),
		releases: (seriesId: number, q?: string) =>
			body(http().GET('/api/v1/series/{id}/releases', searchText(id(seriesId), q))),
		grab: (seriesId: number, release: T.ReleaseResult) =>
			body(http().POST('/api/v1/series/{id}/grab', { ...id(seriesId), body: release })),
		setSeasonMonitored: (seriesId: number, n: number, monitored: boolean) =>
			done(http().PUT('/api/v1/series/{id}/seasons/{season}', { ...season(seriesId, n), body: { monitored } })),
		seasonReleases: (seriesId: number, n: number, q?: string) =>
			body(http().GET('/api/v1/series/{id}/seasons/{season}/releases', searchText(season(seriesId, n), q))),
		searchSeason: (seriesId: number, n: number) =>
			body(http().POST('/api/v1/series/{id}/seasons/{season}/search', season(seriesId, n)))
	},

	episodes: {
		setMonitored: (episodeId: number, monitored: boolean) =>
			body(http().PUT('/api/v1/episodes/{id}', { ...id(episodeId), body: { monitored } })),
		releases: (episodeId: number, q?: string) =>
			body(http().GET('/api/v1/episodes/{id}/releases', searchText(id(episodeId), q))),
		search: (episodeId: number) => body(http().POST('/api/v1/episodes/{id}/search', id(episodeId)))
	},

	queue: {
		list: (f: { movieId?: number; seriesId?: number } = {}) =>
			body(http().GET('/api/v1/queue', { params: { query: { movie_id: f.movieId, series_id: f.seriesId } } })),
		remove: (downloadId: number, deleteData: boolean) =>
			done(
				http().DELETE('/api/v1/queue/{id}', {
					params: { path: { id: downloadId }, query: { delete_data: deleteData || undefined } }
				})
			),
		pause: (downloadId: number) => done(http().POST('/api/v1/queue/{id}/pause', id(downloadId))),
		resume: (downloadId: number) => done(http().POST('/api/v1/queue/{id}/resume', id(downloadId))),
		peers: (downloadId: number) => body(http().GET('/api/v1/queue/{id}/peers', id(downloadId))),
		throughput: () => body(http().GET('/api/v1/queue/throughput'))
	},
	rssSync: () => body(http().POST('/api/v1/rss/sync')),

	settings: {
		general: () => body(http().GET('/api/v1/settings/general')),
		updateGeneral: (tmdb_api_key: string) => body(http().PUT('/api/v1/settings/general', { body: { tmdb_api_key } })),
		network: () => body(http().GET('/api/v1/settings/network')),
		updateNetwork: (expose_lan: boolean) => body(http().PUT('/api/v1/settings/network', { body: { expose_lan } })),
		regenerateAPIKey: () => body(http().POST('/api/v1/settings/api-key/regenerate')),
		changePassword: (current_password: string, new_password: string) =>
			done(http().PUT('/api/v1/settings/password', { body: { current_password, new_password } })),
		downloads: () => body(http().GET('/api/v1/settings/downloads')),
		updateDownloads: (s: T.DownloadSettings) => body(http().PUT('/api/v1/settings/downloads', { body: s })),
		quality: () => body(http().GET('/api/v1/settings/quality')),
		updateQuality: (q: T.QualitySettings) => body(http().PUT('/api/v1/settings/quality', { body: q })),
		vpn: () => body(http().GET('/api/v1/settings/vpn')),
		setVPN: (input: T.VPNInput) => body(http().PUT('/api/v1/settings/vpn', { body: input })),
		removeVPN: () => done(http().DELETE('/api/v1/settings/vpn')),
		testVPN: (input?: T.VPNInput) => body(http().POST('/api/v1/settings/vpn/test', { body: input }))
	},

	rootFolders: {
		list: (media_type: T.MediaType) => body(http().GET('/api/v1/rootfolders', { params: { query: { media_type } } })),
		add: (media_type: T.MediaType, path: string) =>
			body(http().POST('/api/v1/rootfolders', { body: { media_type, path } })),
		setDefault: (folderId: number) => done(http().POST('/api/v1/rootfolders/{id}/default', id(folderId))),
		remove: (folderId: number) => done(http().DELETE('/api/v1/rootfolders/{id}', id(folderId)))
	},

	indexers: {
		list: () => body(http().GET('/api/v1/indexers')),
		definitions: () => body(http().GET('/api/v1/indexers/definitions')),
		reloadDefinitions: () => body(http().POST('/api/v1/indexers/definitions/reload')),
		/** Uploads one definition file, or a zip of them, as the raw request body. */
		uploadDefinitions: (file: Blob) =>
			body(
				http().POST('/api/v1/indexers/definitions', {
					// The spec types a binary body as string; the file is sent as is.
					body: file as unknown as string,
					bodySerializer: (b) => b,
					headers: { 'Content-Type': 'application/octet-stream' }
				})
			),
		removeDefinition: (definitionId: string) =>
			done(http().DELETE('/api/v1/indexers/definitions/{definitionId}', { params: { path: { definitionId } } })),
		create: (input: T.IndexerInput) => body(http().POST('/api/v1/indexers', { body: input })),
		update: (indexerId: number, input: T.IndexerInput) =>
			body(http().PUT('/api/v1/indexers/{id}', { ...id(indexerId), body: input })),
		remove: (indexerId: number) => done(http().DELETE('/api/v1/indexers/{id}', id(indexerId))),
		test: (indexerId: number) => body(http().POST('/api/v1/indexers/{id}/test', id(indexerId)))
	}
};

/** URL of a cached TMDB poster served by arrd. */
export function posterURL(posterPath: string | undefined, size: 'w185' | 'w342' | 'w500' = 'w342'): string | undefined {
	if (!posterPath) return undefined;
	return `/api/v1/posters/${size}/${posterPath.replace(/^\//, '')}`;
}

/** URL of a cached TMDB backdrop (wide art) served by arrd. */
export function backdropURL(backdropPath: string | undefined, size: 'w780' | 'w1280' = 'w780'): string | undefined {
	if (!backdropPath) return undefined;
	return `/api/v1/posters/${size}/${backdropPath.replace(/^\//, '')}`;
}

/** URL of a cached TMDB episode still (16:9) served by arrd. */
export function stillURL(stillPath: string | undefined): string | undefined {
	if (!stillPath) return undefined;
	return `/api/v1/posters/w300/${stillPath.replace(/^\//, '')}`;
}

/** URL of a cached TMDB cast photo served by arrd. */
export function profileURL(profilePath: string | undefined): string | undefined {
	return posterURL(profilePath, 'w185');
}

/** The message of an ApiError about `field`, if that is what err is. */
export function fieldError(err: unknown, field: string): string | undefined {
	return err instanceof ApiError && err.field === field ? err.message : undefined;
}

/** True if err is an ApiError about one of `fields` (shown next to that field instead). */
export function isFieldError(err: unknown, ...fields: string[]): boolean {
	return err instanceof ApiError && !!err.field && fields.includes(err.field);
}
