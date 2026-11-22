/** Formats a byte count with binary units, e.g. "1.5 GiB". */
export function formatBytes(n: number): string {
	if (!Number.isFinite(n) || n < 1024) return `${Math.max(0, Math.round(n || 0))} B`;
	const units = ['KiB', 'MiB', 'GiB', 'TiB', 'PiB'];
	let v = n / 1024;
	let i = 0;
	while (v >= 1024 && i < units.length - 1) {
		v /= 1024;
		i++;
	}
	return `${v.toFixed(1)} ${units[i]}`;
}

/** 0–100, rounded to one decimal. */
export function percent(done: number, total: number): number {
	if (!total) return 0;
	return Math.min(100, Math.round((done / total) * 1000) / 10);
}

/** A short local date ("12 Mar 2024"), or "—". */
export function formatDate(iso?: string): string {
	if (!iso) return '—';
	const d = new Date(iso);
	if (Number.isNaN(d.getTime())) return iso;
	return d.toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' });
}

/** "3 hours ago" style relative time. */
export function timeAgo(iso: string | undefined, now: Date = new Date()): string {
	if (!iso) return 'never';
	const t = new Date(iso);
	// Go encodes "no date" as year 1; treat anything before 1970 as unknown.
	if (Number.isNaN(t.getTime()) || t.getUTCFullYear() < 1970) return 'unknown';
	const s = Math.round((now.getTime() - new Date(iso).getTime()) / 1000);
	if (s < 60) return 'just now';
	const units: [number, string][] = [
		[60 * 60 * 24, 'day'],
		[60 * 60, 'hour'],
		[60, 'minute']
	];
	for (const [secs, name] of units) {
		if (s >= secs) {
			const n = Math.floor(s / secs);
			return `${n} ${name}${n === 1 ? '' : 's'} ago`;
		}
	}
	return 'just now';
}

/** "S01E02" */
export function episodeCode(season: number, episode: number): string {
	return `S${String(season).padStart(2, '0')}E${String(episode).padStart(2, '0')}`;
}

/** A transfer rate in bytes per second, e.g. "3.1 MiB/s". */
export function formatRate(bytesPerSecond: number): string {
	return `${formatBytes(bytesPerSecond)}/s`;
}

/** Time left as its two largest units, e.g. "4m 05s", "1h 20m", "2d 3h"; "—" when unknown (0). */
export function formatETA(seconds: number): string {
	if (!Number.isFinite(seconds) || seconds <= 0) return '—';
	const s = Math.ceil(seconds);
	const d = Math.floor(s / 86_400);
	const h = Math.floor((s % 86_400) / 3_600);
	const m = Math.floor((s % 3_600) / 60);
	const sec = s % 60;
	if (d > 0) return `${d}d ${h}h`;
	if (h > 0) return `${h}h ${String(m).padStart(2, '0')}m`;
	if (m > 0) return `${m}m ${String(sec).padStart(2, '0')}s`;
	return `${sec}s`;
}
