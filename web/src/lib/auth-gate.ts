import { HOME } from './links';
import type { Session } from './types';

const PUBLIC_WHEN_SIGNED_OUT = ['/login'];

export { HOME };

/**
 * Decides where the app shell should send the user, or null to stay.
 * - not set up → /setup (keeping ?token=)
 * - set up, signed out → /login?next=<where they were going>
 * - signed in on /setup or /login → next or HOME
 */
export function gateRedirect(session: Session, url: URL): string | null {
	const path = url.pathname;
	if (!session.setup_complete) {
		if (path === '/setup') return null;
		const token = url.searchParams.get('token');
		return token ? `/setup?token=${encodeURIComponent(token)}` : '/setup';
	}
	if (!session.authenticated) {
		if (PUBLIC_WHEN_SIGNED_OUT.includes(path)) return null;
		const next = path === '/' || path === '/setup' ? '' : `?next=${encodeURIComponent(path + url.search)}`;
		return `/login${next}`;
	}
	if (path === '/setup' || path === '/login') {
		return safeNext(url.searchParams.get('next')) ?? HOME;
	}
	if (path === '/') return HOME;
	return null;
}

/** Only same-app relative paths are allowed as a post-login target. */
export function safeNext(next: string | null): string | null {
	if (!next || !next.startsWith('/') || next.startsWith('//') || next.startsWith('/\\')) return null;
	return next;
}
