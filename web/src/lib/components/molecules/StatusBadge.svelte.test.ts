import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import { statusLabel } from '#lib/domain/status.js';
import StatusBadge from './StatusBadge.svelte';

describe('StatusBadge', () => {
	it('labels every status', () => {
		for (const [status, label] of Object.entries(statusLabel)) {
			const { unmount } = render(StatusBadge, { props: { status: status as keyof typeof statusLabel } });
			expect(screen.getByText(label)).toBeInTheDocument();
			unmount();
		}
	});
});
