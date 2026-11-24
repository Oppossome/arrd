import type { DownloadSettings, ImportMode, IndexerInput, SetupInput, SetupStatus, SettingValues } from '../api/types';
import { toVPNInput } from './settingsForm';

/** Everything the onboarding wizard collects, across its steps. */
export interface SetupDraft {
	token: string;
	password: string;
	passwordConfirm: string;
	tmdbKey: string;
	incompleteDir: string;
	completeDir: string;
	movieRoot: string;
	tvRoot: string;
	createMissing: boolean;
	importMode: ImportMode;
	/** VPN provider id; empty for no VPN. */
	vpnProvider: string;
	vpnValues: SettingValues;
	/** Public IP from a successful test of the current VPN settings. */
	vpnTestedIP: string;
	/** One indexer per site, in the order they were switched on. */
	indexers: IndexerInput[];
}

export const SETUP_STEPS = ['Account', 'TMDB', 'Folders', 'VPN', 'Indexers', 'Review'] as const;

export function newDraft(token: string, suggested?: SetupStatus['suggested']): SetupDraft {
	return {
		token,
		password: '',
		passwordConfirm: '',
		tmdbKey: '',
		incompleteDir: suggested?.incomplete_dir ?? '',
		completeDir: suggested?.complete_dir ?? '',
		movieRoot: suggested?.movie_root ?? '',
		tvRoot: suggested?.tv_root ?? '',
		createMissing: true,
		importMode: 'hardlink',
		vpnProvider: '',
		vpnValues: {},
		vpnTestedIP: '',
		indexers: []
	};
}

export const MIN_PASSWORD = 8;

/** A reason the account step cannot continue, or null. */
export function passwordProblem(d: Pick<SetupDraft, 'password' | 'passwordConfirm' | 'token'>): string | null {
	if (!d.token.trim()) return 'Enter the setup token printed in the arrd log.';
	if ([...d.password].length < MIN_PASSWORD) return `The password must be at least ${MIN_PASSWORD} characters.`;
	if (d.password !== d.passwordConfirm) return 'The passwords do not match.';
	return null;
}

/** A reason the VPN step cannot continue, or null. */
export function vpnProblem(d: Pick<SetupDraft, 'vpnProvider' | 'vpnTestedIP'>): string | null {
	if (!d.vpnProvider) return null;
	if (!d.vpnTestedIP) return 'Test the VPN connection before continuing.';
	return null;
}

/** Converts the draft into the API's setup request; the wizard always sends every download setting. */
export function toSetupInput(d: SetupDraft): SetupInput & { downloads: DownloadSettings } {
	const hasVPN = d.vpnProvider !== '';
	return {
		admin_password: d.password,
		tmdb_api_key: d.tmdbKey.trim(),
		downloads: {
			incomplete_dir: d.incompleteDir.trim(),
			complete_dir: d.completeDir.trim(),
			import_mode: d.importMode
		},
		vpn: hasVPN ? toVPNInput(d.vpnProvider, d.vpnValues) : undefined,
		movie_root: d.movieRoot.trim() || undefined,
		tv_root: d.tvRoot.trim() || undefined,
		indexers: d.indexers,
		create_missing: d.createMissing
	};
}
