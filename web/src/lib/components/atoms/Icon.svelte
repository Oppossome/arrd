<script lang="ts" module>
	// A small inline icon set (24×24 stroke paths), so no icon font or external assets.
	export const iconPaths = {
		film: 'M4 4h16v16H4zM4 9h16M4 15h16M9 4v16M15 4v16',
		tv: 'M3 6h18v12H3zM8 21h8M12 18v3',
		activity: 'M3 12h4l3-8 4 16 3-8h4',
		settings: 'M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-2.8 1.2V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-2.8-1.2l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0-1.2-2.8H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.2-2.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 2.8-1.2V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 2.8 1.2l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0 1.2 2.8H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1.1z',
		search: 'M11 19a8 8 0 1 0 0-16 8 8 0 0 0 0 16zM21 21l-4.3-4.3',
		plus: 'M12 5v14M5 12h14',
		trash: 'M3 6h18M8 6V4h8v2M6 6l1 14h10l1-14',
		refresh: 'M21 12a9 9 0 1 1-3-6.7L21 8M21 3v5h-5',
		x: 'M18 6 6 18M6 6l12 12',
		check: 'M20 6 9 17l-5-5',
		chevron: 'm9 18 6-6-6-6',
		'chevron-left': 'm15 18-6-6 6-6',
		folder: 'M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z',
		up: 'm18 15-6-6-6 6',
		copy: 'M9 9h11v11H9zM5 15H4V4h11v1',
		logout: 'M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4M16 17l5-5-5-5M21 12H9',
		download: 'M12 3v12M7 10l5 5 5-5M5 21h14',
		shield: 'M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z',
		alert: 'M12 9v4M12 17h.01M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z',
		pause: 'M8 5v14M16 5v14',
		play: 'M7 4v16l13-8z',
		users: 'M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM22 21v-2a4 4 0 0 0-3-3.9M16 3.1a4 4 0 0 1 0 7.8'
	} as const;
	export type IconName = keyof typeof iconPaths;
</script>

<script lang="ts">
	interface Props {
		name: IconName;
		size?: number;
		label?: string;
	}
	let { name, size = 18, label }: Props = $props();
</script>

<svg
	width={size}
	height={size}
	viewBox="0 0 24 24"
	fill="none"
	stroke="currentColor"
	stroke-width="2"
	stroke-linecap="round"
	stroke-linejoin="round"
	role={label ? 'img' : undefined}
	aria-label={label}
	aria-hidden={label ? undefined : 'true'}
>
	<path d={iconPaths[name]} />
</svg>

<style>
	svg {
		flex-shrink: 0;
	}
</style>
