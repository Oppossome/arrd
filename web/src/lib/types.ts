// arrd's JSON API types, generated from tools/openapi.yaml into
// api.gen.ts (`npm run gen:api`). This module only gives them short names;
// change the spec, not these aliases.

import type { components, operations } from './api.gen';

type Schemas = components['schemas'];

export type ImportMode = Schemas['ImportMode'];
export type MediaType = Schemas['MediaType'];
export type MovieStatus = Schemas['MovieStatus'];
export type DownloadStatus = Schemas['DownloadStatus'];
export type MonitorMode = Schemas['MonitorMode'];

export type ApiErrorBody = Schemas['Error'];
export type Session = Schemas['Session'];
export type RuntimeStatus = Schemas['RuntimeStatus'];

export type Movie = Schemas['Movie'];
export type SeasonStatus = Schemas['SeasonStatus'];
export type Series = Schemas['Series'];
export type Episode = Schemas['Episode'];
/** A TMDB lookup result (not yet in the library). */
export type LookupMovie = Schemas['LookupMovie'];
export type LookupSeries = Schemas['LookupSeries'];
export type MovieRecommendations = Schemas['MovieRecommendations'];
export type SeriesRecommendations = Schemas['SeriesRecommendations'];
export type MovieList = Schemas['MovieList'];
export type SeriesList = Schemas['SeriesList'];
export type DiscoverSort = Schemas['DiscoverSort'];
export type Genre = Schemas['Genre'];
export type Language = Schemas['Language'];
export type MovieBrowsePage = Schemas['MovieBrowsePage'];
export type SeriesBrowsePage = Schemas['SeriesBrowsePage'];
/** A browse of TMDB's catalogue: every genre and language when absent. */
export type BrowseQuery = NonNullable<operations['browseMovies']['parameters']['query']>;
export type AddMovieRequest = Schemas['AddMovieRequest'];
export type UpdateMovieRequest = Schemas['UpdateMovieRequest'];
export type AddSeriesRequest = Schemas['AddSeriesRequest'];
export type UpdateSeriesRequest = Schemas['UpdateSeriesRequest'];

export type Progress = Schemas['Progress'];
export type Download = Schemas['Download'];
export type CountryCount = Schemas['CountryCount'];
export type Peer = Schemas['Peer'];
export type Throughput = Schemas['Throughput'];
export type ThroughputSample = Schemas['ThroughputSample'];
export type ReleaseResult = Schemas['ReleaseResult'];
export type IndexerFailure = Schemas['IndexerFailure'];
export type SearchResult = Schemas['SearchResult'];
export type AutoSearchResult = Schemas['AutoSearchResult'];
export type RSSResult = Schemas['RSSResult'];

export type DownloadSettings = Schemas['DownloadSettings'];
export type SettingsUpdate = Schemas['SettingsUpdate'];
export type QualitySettings = Schemas['QualitySettings'];
export type GeneralSettings = Schemas['GeneralSettings'];
export type RootFolder = Schemas['RootFolder'];

export type SearchCap = Schemas['SearchCap'];
export type Indexer = Schemas['Indexer'];
export type IndexerInput = Schemas['IndexerInput'];
/** A site arrd can search (from a Cardigann definition) and the settings it needs. */
export type IndexerDefinition = Schemas['IndexerDefinition'];

/** How an integration setting is entered. */
export type SettingFieldKind = Schemas['SettingFieldKind'];
/** One setting an integration (VPN provider, indexer site) needs, declared by the server. */
export type SettingField = Schemas['SettingField'];
export type SettingOption = Schemas['SettingOption'];
/** Settings by field key; toggles are "true" / "false". */
export type SettingValues = Schemas['SettingValues'];
/** Anything whose settings form is declared by the server. */
export interface SettingsSchema {
	fields: SettingField[];
}

export type VPNProvider = Schemas['VPNProvider'];

export type VPNInput = Schemas['VPNInput'];
export type VPNSettings = Schemas['VPNSettings'];
export type VPNTest = Schemas['VPNTest'];

export type SetupStatus = Schemas['SetupStatus'];
export type SetupInput = Schemas['SetupInput'];
export type SetupResult = Schemas['SetupResult'];
export type PathsInput = Schemas['PathsInput'];
export type PathsCheck = Schemas['PathsCheck'];
export type DirListing = Schemas['DirListing'];

/** Narrows TMDB titles by genre (any of them) and original language. */
export interface TitleFilter {
	/** TMDB genre ids; every genre when empty. */
	genres: number[];
	/** ISO 639-1 code; every language when absent. */
	language?: string;
}
