import { redirect } from '@sveltejs/kit';
import { queryClient } from '#lib/api/app-client.js';
import { gateRedirect } from '#lib/nav/auth-gate.js';
import { queries } from '#lib/api/query.js';
import type { LayoutLoad } from './$types';

// A client-only single-page app embedded in the arrd binary.
export const ssr = false;
export const prerender = false;

/** Sends the user to setup, login or the app, depending on their session. */
export const load: LayoutLoad = async ({ url }) => {
	const session = await queryClient.fetchQuery({ ...queries.session(), staleTime: 0 });
	const target = gateRedirect(session, url);
	if (target && target !== url.pathname + url.search) redirect(307, target);
	return { session };
};
