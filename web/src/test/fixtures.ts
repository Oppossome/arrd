// Builders for API objects in tests. Titles, hashes and addresses are
// invented (addresses from the documentation ranges).

import type { Download, Peer, Progress, Throughput } from '#lib/types.js';

let nextId = 100;

export function progress(over: Partial<Progress> = {}): Progress {
	return {
		info_hash: 'f'.repeat(40),
		name: 'Release',
		have_info: true,
		bytes_completed: 0,
		bytes_total: 1000,
		active_peers: 0,
		total_peers: 0,
		seeders: 0,
		complete: false,
		download_rate: 0,
		upload_rate: 0,
		eta_seconds: 0,
		countries: [],
		...over
	};
}

export function download(over: Partial<Download> = {}): Download {
	return {
		id: nextId++,
		info_hash: 'f'.repeat(40),
		release_title: 'Lantern.Bay.2021.1080p.WEB-DL-QUILL',
		quality: 'WEBDL-1080p',
		size: 1000,
		status: 'downloading',
		paused: false,
		added_at: '2026-11-20T10:00:00Z',
		...over
	};
}

export function peer(over: Partial<Peer> = {}): Peer {
	return {
		ip: '198.51.100.7',
		port: 51413,
		country: 'DE',
		client: 'qBittorrent 5.0.1',
		source: 'tracker',
		download_rate: 0,
		upload_rate: 0,
		progress: 0,
		...over
	};
}

export function throughput(over: Partial<Throughput> = {}): Throughput {
	return { download_rate: 0, upload_rate: 0, peak_download_rate: 0, active: 0, history: [], ...over };
}
