import { fireEvent, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { mockFetch, renderWithClient } from '../../../test/helpers';
import type { Episode, LookupSeries, SeasonDetails, Series } from '#lib/types.js';
import SeasonBrowser from './SeasonBrowser.svelte';

afterEach(() => vi.unstubAllGlobals());

const show = (over: Partial<LookupSeries> = {}): LookupSeries => ({
	tmdb_id: 800001,
	title: 'Harbor Watch',
	original_title: 'Harbor Watch',
	year: 2019,
	overview: 'A fictional coastal drama.',
	seasons: [
		{ season_number: 0, episode_count: 1 },
		{ season_number: 1, episode_count: 2, name: 'Season 1', air_date: '2019-09-01' },
		{ season_number: 2, episode_count: 1, name: 'Season 2', air_date: '2024-06-10' },
		{ season_number: 3, episode_count: 0 } // announced, no episodes yet
	],
	...over
});

const season = (n: number, titles: string[]): SeasonDetails => ({
	season_number: n,
	name: `Season ${n}`,
	overview: '',
	episodes: titles.map((title, i) => ({
		season: n,
		episode: i + 1,
		title,
		overview: `${title} synopsis.`,
		air_date: '2019-09-01',
		runtime: 47,
		still_path: `/still${n}${i + 1}.jpg`
	}))
});

describe('SeasonBrowser for a show not in the library', () => {
	it('tabs through seasons and asks for an episode or season download', async () => {
		mockFetch({
			'GET /api/v1/discover/series/800001': () => Response.json(show()),
			'GET /api/v1/discover/series/800001/seasons/2': () => Response.json(season(2, ['Return'])),
			'GET /api/v1/discover/series/800001/seasons/1': () => Response.json(season(1, ['Pilot', 'Low Tide']))
		});
		const ondownload = vi.fn();
		renderWithClient(SeasonBrowser, { tmdbId: 800001, ondownload });

		// Seasons in order, specials last, empty seasons left out; the latest is open.
		const tabs = await screen.findAllByRole('tab');
		expect(tabs.map((t) => t.textContent)).toEqual(['S1', 'S2', 'Specials']);
		expect(screen.getByRole('tab', { name: 'S2' })).toHaveAttribute('aria-selected', 'true');
		expect(await screen.findByText('Return')).toBeInTheDocument();

		await fireEvent.click(screen.getByRole('tab', { name: 'S1' }));
		const lowTide = await screen.findByRole('article', { name: 'S01E02 Low Tide' });
		expect(within(lowTide).getByText('Low Tide synopsis.')).toBeInTheDocument();
		expect(within(lowTide).getByText('47m')).toBeInTheDocument();

		await fireEvent.click(within(lowTide).getByRole('button', { name: 'Download S01E02' }));
		expect(ondownload).toHaveBeenLastCalledWith({
			title: 'Harbor Watch · S01E02 Low Tide',
			source: { kind: 'discoverEpisode', tmdbId: 800001, season: 1, episode: 2 }
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Download season' }));
		expect(ondownload).toHaveBeenLastCalledWith({
			title: 'Harbor Watch · Season 1',
			source: { kind: 'discoverSeason', tmdbId: 800001, season: 1 }
		});
		// Library controls only appear for library shows.
		expect(screen.queryByRole('button', { name: 'Search' })).not.toBeInTheDocument();
	});
});

describe('SeasonBrowser for a library show', () => {
	it('merges library status into the episodes and downloads through the library', async () => {
		const series = {
			id: 4,
			tmdb_id: 800001,
			title: 'Harbor Watch',
			seasons: [
				{ season: 1, episodes: 2, downloaded: 1, monitored: 2, aired: 2, complete: false },
				{ season: 2, episodes: 1, downloaded: 0, monitored: 1, aired: 1, complete: false }
			]
		} as unknown as Series;
		const episode = (n: number, over: Partial<Episode> = {}): Episode => ({
			id: 40 + n,
			series_id: 4,
			season: 1,
			episode: n,
			title: `Episode ${n}`,
			air_date: '2019-09-01',
			monitored: true,
			downloading: false,
			...over
		});
		mockFetch({
			'GET /api/v1/series/4': () => Response.json(series),
			'GET /api/v1/series/4/episodes': () =>
				Response.json([episode(1, { file_path: '/tv/a.mkv', file_quality: 'WEBDL-1080p' }), episode(2)]),
			'GET /api/v1/discover/series/800001': () => Response.json(show({ library_id: 4 })),
			'GET /api/v1/discover/series/800001/seasons/2': () => Response.json(season(2, ['Return'])),
			'GET /api/v1/discover/series/800001/seasons/1': () => Response.json(season(1, ['Pilot', 'Low Tide']))
		});
		const ondownload = vi.fn();
		renderWithClient(SeasonBrowser, { seriesId: 4, ondownload });

		await fireEvent.click(await screen.findByRole('tab', { name: 'S1' }));
		const pilot = await screen.findByRole('article', { name: 'S01E01 Pilot' });
		expect(within(pilot).getByText('WEBDL-1080p')).toBeInTheDocument();
		expect(within(screen.getByRole('article', { name: 'S01E02 Low Tide' })).getByText('Missing')).toBeInTheDocument();
		expect(screen.getByText('1/2 downloaded')).toBeInTheDocument();

		await fireEvent.click(within(pilot).getByRole('button', { name: 'Download S01E01' }));
		expect(ondownload).toHaveBeenLastCalledWith({
			title: 'Harbor Watch · S01E01 Pilot',
			source: { kind: 'episode', seriesId: 4, episodeId: 41 }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Download season' }));
		expect(ondownload).toHaveBeenLastCalledWith({
			title: 'Harbor Watch · Season 1',
			source: { kind: 'season', seriesId: 4, season: 1 }
		});
	});
});
