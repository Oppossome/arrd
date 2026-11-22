// Atomic design import rules: components may only depend on lower levels,
// and only organisms talk to the server (through #lib/query.js).

/** @param {string} name */
const level = (name) => [`#lib/components/${name}/*`, `**/components/${name}/**`, `../${name}/**`];
const noData = [
	{ name: '@tanstack/svelte-query', message: 'Only organisms may use queries or mutations.' },
	{ name: '#lib/query.js', message: 'Only organisms may use queries or mutations.' },
	{ name: '#lib/app-client.js', message: 'Only the root layout wires the query client.' },
	{ name: '#lib/api.js', importNames: ['api', 'request'], message: 'Only organisms call the API (via #lib/query.js).' }
];
/**
 * @param {string[]} files
 * @param {string[]} levels
 * @param {{ name: string; importNames?: string[]; message: string }[]} paths
 * @param {string} why
 * @returns {import('eslint').Linter.Config}
 */
const restrict = (files, levels, paths, why) => ({
	files,
	rules: {
		'no-restricted-imports': [
			'error',
			{
				paths,
				patterns: levels.length ? [{ group: levels.flatMap(level), message: why }] : []
			}
		]
	}
});

export const atomicDesignRules = [
	restrict(
		['src/lib/components/atoms/**'],
		['molecules', 'organisms', 'templates'],
		noData,
		'Atoms are the lowest level: they may not import molecules, organisms or templates.'
	),
	restrict(
		['src/lib/components/molecules/**'],
		['organisms', 'templates'],
		noData,
		'Molecules may only import atoms (and other molecules).'
	),
	restrict(
		['src/lib/components/templates/**'],
		['organisms'],
		noData,
		'Templates are layouts: they may import atoms and molecules, not organisms.'
	),
	restrict(
		['src/lib/components/organisms/**'],
		['templates', 'organisms'],
		[],
		'Organisms may import molecules and atoms, not templates or other organisms (compose them in a page instead).'
	),
	restrict(
		['src/routes/**/*.svelte'],
		['atoms', 'molecules'],
		[
			{ name: '#lib/api.js', importNames: ['api', 'request'], message: 'Pages compose organisms; they make no API calls.' },
			{ name: '#lib/query.js', message: 'Pages compose organisms; they make no API calls.' }
		],
		'Pages compose templates and organisms only.'
	)
];
