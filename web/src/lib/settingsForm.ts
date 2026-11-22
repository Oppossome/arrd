// Helpers for schema-driven settings forms (VPN providers, indexer sites).
// Integrations declare their fields on the server; nothing here knows about
// a specific one.

import type { SettingField, SettingValues, SettingsSchema, VPNInput } from './types';

/** Starting values: saved values, else each field's default, else toggles off and everything else empty. */
export function initialValues(p: SettingsSchema, saved?: SettingValues): SettingValues {
	const out: SettingValues = {};
	for (const f of p.fields) {
		out[f.key] = saved?.[f.key] ?? f.default ?? (f.kind === 'toggle' ? 'false' : '');
	}
	return out;
}

/** Fields whose choices must be loaded from the server. */
export function optionFields(p: SettingsSchema | undefined): SettingField[] {
	return p?.fields.filter((f) => f.kind === 'select' && f.options) ?? [];
}

/**
 * The first required field without a value, or null when the form can be
 * submitted. A secret already saved on the server counts as filled.
 */
export function missingField(p: SettingsSchema, values: SettingValues, secretsSet: readonly string[] = []): SettingField | null {
	for (const f of p.fields) {
		if (!f.required) continue;
		if ((values[f.key] ?? '').trim() !== '') continue;
		if (f.secret && secretsSet.includes(f.key)) continue;
		return f;
	}
	return null;
}

/** Values to send: empty values are left out (an empty secret keeps the saved one). */
export function compactValues(values: SettingValues): SettingValues {
	const out: SettingValues = {};
	for (const [k, v] of Object.entries(values)) {
		if (v !== '') out[k] = v;
	}
	return out;
}

/** The request body for a VPN provider and its form values. */
export function toVPNInput(provider: string, values: SettingValues): VPNInput {
	return { provider, values: compactValues(values) };
}

/** Whether the schema has a required field its defaults leave empty, so it must be filled in before use. */
export function needsSettings(p: SettingsSchema): boolean {
	return missingField(p, initialValues(p)) !== null;
}
