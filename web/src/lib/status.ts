import type { DownloadStatus, MovieStatus } from './types';

export type Tone = 'neutral' | 'info' | 'success' | 'warning' | 'danger';

export const statusTone: Record<MovieStatus | DownloadStatus, Tone> = {
	missing: 'warning',
	downloading: 'info',
	downloaded: 'success',
	unmonitored: 'neutral',
	completed: 'info',
	imported: 'success',
	failed: 'danger'
};

export const statusLabel: Record<MovieStatus | DownloadStatus, string> = {
	missing: 'Missing',
	downloading: 'Downloading',
	downloaded: 'Downloaded',
	unmonitored: 'Unmonitored',
	completed: 'Importing',
	imported: 'Imported',
	failed: 'Failed'
};
