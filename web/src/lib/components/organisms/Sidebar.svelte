<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { createQuery } from '@tanstack/svelte-query';
	import { Icon, type IconName } from '#lib/components/atoms/index.js';
	import { HOME, LIBRARY } from '#lib/links.js';
	import { queries, useMutation } from '#lib/query.js';
	import ThroughputWidget from './ThroughputWidget.svelte';

	const links: { href: string; label: string; icon: IconName }[] = [
		{ href: '/movies', label: 'Movies', icon: 'film' },
		{ href: '/shows', label: 'Shows', icon: 'tv' },
		{ href: LIBRARY, label: 'Library', icon: 'folder' },
		{ href: '/settings', label: 'Settings', icon: 'settings' }
	];
	const logout = useMutation('logout');
	// Shared with ThroughputWidget: one request for both.
	const throughput = createQuery(() => queries.throughput());
	const activeCount = $derived(throughput.data?.active ?? 0);
	const active = (href: string) => page.url.pathname === href || page.url.pathname.startsWith(href + '/');
</script>

<nav class="nav" aria-label="Main">
	<a class="brand" href={HOME}>arrd</a>
	<ul>
		{#each links as link (link.href)}
			<li>
				<a href={link.href} class:active={active(link.href)} aria-current={active(link.href) ? 'page' : undefined}>
					<Icon name={link.icon} />
					{link.label}
					{#if link.href === LIBRARY && activeCount > 0}
						<span class="count" aria-label="{activeCount} downloading">{activeCount}</span>
					{/if}
				</a>
			</li>
		{/each}
	</ul>
	<div class="footer">
		<ThroughputWidget />
		<button class="logout" type="button" onclick={() => logout.mutate(undefined, { onSuccess: () => void goto('/login') })}>
			<Icon name="logout" /> Log out
		</button>
	</div>
</nav>

<style>
	.nav {
		position: sticky;
		top: 0;
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
		padding: var(--space-5) var(--space-3);
		height: 100vh;
	}
	.brand {
		padding: 0 var(--space-3);
		color: var(--color-text);
		font-size: var(--text-xl);
		font-weight: var(--weight-bold);
		text-decoration: none;
	}
	ul {
		display: grid;
		gap: var(--space-1);
		margin: 0;
		padding: 0;
		list-style: none;
	}
	a:not(.brand),
	.logout {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-2) var(--space-3);
		border-radius: var(--radius-md);
		color: var(--color-text-muted);
		text-decoration: none;
		font-size: var(--text-sm);
	}
	a:not(.brand):hover,
	.logout:hover {
		color: var(--color-text);
		background: var(--color-surface-raised);
	}
	a.active {
		color: var(--color-text);
		background: var(--color-surface-raised);
	}
	.footer {
		display: grid;
		gap: var(--space-3);
		margin-top: auto;
	}
	.count {
		margin-left: auto;
		padding: 0 var(--space-2);
		border-radius: var(--radius-pill);
		background: var(--color-accent);
		color: var(--color-on-accent);
		font-size: var(--text-xs);
		font-weight: var(--weight-bold);
	}
	.logout {
		border: 0;
		background: transparent;
		font: inherit;
		font-size: var(--text-sm);
		cursor: pointer;
	}
	@media (max-width: 48rem) {
		.nav {
			position: static;
			height: auto;
			flex-direction: row;
			flex-wrap: wrap;
			align-items: center;
		}
		ul {
			display: flex;
		}
		.footer {
			margin: 0 0 0 auto;
			display: flex;
			align-items: center;
		}
	}
</style>
