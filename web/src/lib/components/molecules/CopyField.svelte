<script lang="ts">
	import { IconButton, Input } from '#lib/components/atoms/index.js';

	interface Props {
		value: string;
		label: string;
	}
	let { value, label }: Props = $props();
	let copied = $state(false);

	async function copy() {
		await navigator.clipboard.writeText(value);
		copied = true;
		setTimeout(() => (copied = false), 1500);
	}
</script>

<div class="copy">
	<Input value={value} readonly monospace aria-label={label} />
	<IconButton icon={copied ? 'check' : 'copy'} label={copied ? 'Copied' : `Copy ${label}`} onclick={copy} />
</div>

<style>
	.copy {
		display: flex;
		gap: var(--space-2);
		align-items: center;
	}
</style>
