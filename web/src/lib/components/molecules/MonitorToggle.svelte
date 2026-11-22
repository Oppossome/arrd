<script lang="ts">
	import { Toggle } from '#lib/components/atoms/index.js';

	interface Props {
		monitored: boolean;
		what: string;
		disabled?: boolean;
		onchange: (monitored: boolean) => void;
	}
	let { monitored, what, disabled = false, onchange }: Props = $props();
</script>

<Toggle checked={monitored} label={monitored ? `Stop monitoring ${what}` : `Monitor ${what}`} {disabled} {onchange} />
