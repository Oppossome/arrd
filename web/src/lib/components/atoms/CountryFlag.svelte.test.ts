import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import { countryName } from '#lib/utils/countries.js';
import CountryFlag from './CountryFlag.svelte';

describe('CountryFlag', () => {
	it('names the country for assistive tech', () => {
		render(CountryFlag, { props: { code: 'DE', showCode: true } });
		expect(screen.getByRole('img', { name: countryName('DE') })).toHaveTextContent('🇩🇪');
		expect(screen.getByText('DE')).toBeInTheDocument();
	});

	it('marks unknown locations', () => {
		render(CountryFlag, { props: { code: '' } });
		expect(screen.getByRole('img', { name: 'Unknown location' })).toBeInTheDocument();
	});
});
