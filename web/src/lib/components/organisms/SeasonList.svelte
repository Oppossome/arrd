<script lang="ts">
	import type { Snippet } from 'svelte';
	import { createQuery } from '@tanstack/svelte-query';
	import { Badge, Button, Icon, Spinner, Text } from '#lib/components/atoms/index.js';
	import { ErrorMessage, MonitorToggle } from '#lib/components/molecules/index.js';
	import { queries, useMutation } from '#lib/query.js';
	import { toasts } from '#lib/toast.svelte.js';
	import type { SeasonStatus } from '#lib/types.js';

	interface Props {
		seriesId: number;
		/** Body of an expanded season (the page fills it with episode and release organisms). */
		season: Snippet<[SeasonStatus]>;
	}
	let { seriesId, season }: Props = $props();

	const series = createQuery(() => queries.series(seriesId));
	const setMonitored = useMutation('setSeasonMonitored');
	const searchSeason = useMutation('searchSeason');
	let open = $state<Record<number, boolean>>({});
	let searching = $state<number | null>(null);

	// Newest season first; specials last.
	const seasons = $derived(
		[...(series.data?.seasons ?? [])].sort((a, b) => (a.season === 0 ? 1 : b.season === 0 ? -1 : b.season - a.season))
	);

	function search(n: number) {
		searching = n;
		searchSeason.mutate(
			{ id: seriesId, season: n },
			{
				onSuccess: (results) => {
					const grabbed = results.filter((r) => r.grabbed).length;
					toasts.push(grabbed ? 'success' : 'info', grabbed ? `Grabbed ${grabbed} release(s)` : 'No acceptable releases found');
				},
				onSettled: () => (searching = null)
			}
		);
	}
</script>

{#if series.isPending}
	<Spinner label="Loading seasons" />
{:else if series.isError}
	<ErrorMessage error={series.error} />
{:else}
	<div class="seasons">
		{#each seasons as s (s.season)}
			<section class="season">
				<div class="summary">
					<button
						type="button"
						class="expand"
						aria-expanded={!!open[s.season]}
						onclick={() => (open[s.season] = !open[s.season])}
					>
						<span class="chev" class:rot={open[s.season]}><Icon name="chevron" /></span>
						<Text as="span">{s.season === 0 ? 'Specials' : `Season ${s.season}`}</Text>
					</button>
					<Badge tone={s.complete ? 'success' : s.downloaded > 0 ? 'info' : 'neutral'}>{s.downloaded}/{s.aired} aired</Badge>
					<Text size="xs" tone="muted" as="span">{s.episodes} episodes</Text>
					<span class="spacer"></span>
					<MonitorToggle
						monitored={s.monitored > 0}
						what={s.season === 0 ? 'specials' : `season ${s.season}`}
						onchange={(v) => setMonitored.mutate({ seriesId, season: s.season, monitored: v })}
					/>
					<Button size="sm" loading={searching === s.season} onclick={() => search(s.season)}>Search</Button>
				</div>
				{#if open[s.season]}
					<div class="body">{@render season(s)}</div>
				{/if}
			</section>
		{/each}
	</div>
{/if}

<style>
	.seasons {
		display: grid;
		gap: var(--space-3);
	}
	.season {
		border: 1px solid var(--color-border);
		border-radius: var(--radius-lg);
		background: var(--color-surface);
	}
	.summary {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-3) var(--space-4);
	}
	.expand {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		border: 0;
		background: transparent;
		color: var(--color-text);
		font: inherit;
		cursor: pointer;
	}
	.chev {
		display: inline-flex;
		transition: transform var(--transition-fast);
	}
	.rot {
		transform: rotate(90deg);
	}
	.spacer {
		flex: 1;
	}
	.body {
		display: grid;
		gap: var(--space-4);
		padding: 0 var(--space-4) var(--space-4);
	}
</style>
