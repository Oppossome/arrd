<script lang="ts">
	import { Badge } from '#lib/components/atoms/index.js';
	import { statusLabel, statusTone } from '#lib/domain/status.js';
	import type { DownloadStatus, MovieStatus } from '#lib/api/types.js';

	let { status }: { status: MovieStatus | DownloadStatus } = $props();
</script>

<Badge tone={statusTone[status]}>{statusLabel[status]}</Badge>
