<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Button, Heading, Text } from '#lib/components/atoms/index.js';

	interface Props {
		title: string;
		steps: readonly string[];
		current: number;
		busy?: boolean;
		nextLabel?: string;
		onback?: () => void;
		onnext: () => void;
		children: Snippet;
	}
	let { title, steps, current, busy = false, nextLabel = 'Continue', onback, onnext, children }: Props = $props();
</script>

<div class="page">
	<div class="wizard">
		<header class="header">
			<Heading level={1}>{title}</Heading>
			<ol class="steps" aria-label="Setup steps">
				{#each steps as step, i (step)}
					<li class:done={i < current} class:current={i === current} aria-current={i === current ? 'step' : undefined}>
						<Text size="xs" as="span">{i + 1}. {step}</Text>
					</li>
				{/each}
			</ol>
		</header>
		<!-- Not a <form>: steps may contain their own forms (e.g. adding an indexer). -->
		<section class="body" aria-label={steps[current]}>
			{@render children()}
			<footer class="actions">
				{#if onback}<Button onclick={onback} disabled={busy}>Back</Button>{:else}<span></span>{/if}
				<Button variant="primary" loading={busy} onclick={onnext}>{nextLabel}</Button>
			</footer>
		</section>
	</div>
</div>

<style>
	.page {
		display: grid;
		justify-items: center;
		min-height: 100vh;
		padding: var(--space-6) var(--space-4);
	}
	.wizard {
		display: grid;
		gap: var(--space-5);
		width: min(44rem, 100%);
		align-content: start;
	}
	.header {
		display: grid;
		gap: var(--space-4);
	}
	.steps {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-2);
		margin: 0;
		padding: 0;
		list-style: none;
	}
	.steps li {
		padding: var(--space-1) var(--space-3);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-pill);
		color: var(--color-text-muted);
	}
	.steps li.done {
		color: var(--color-success);
		border-color: var(--color-success);
	}
	.steps li.current {
		color: var(--color-text);
		border-color: var(--color-accent);
	}
	.body {
		display: grid;
		gap: var(--space-4);
		padding: var(--space-5);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-lg);
		background: var(--color-surface);
	}
	.actions {
		display: flex;
		justify-content: space-between;
		gap: var(--space-2);
		padding-top: var(--space-2);
	}
</style>
