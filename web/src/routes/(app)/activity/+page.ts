import { redirect } from '@sveltejs/kit';
import { LIBRARY } from '#lib/nav/links.js';

// Downloads now live on the library page.
export function load() {
	redirect(308, LIBRARY);
}
