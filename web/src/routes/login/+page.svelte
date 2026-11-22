<script lang="ts">
	import { page } from '$app/state';
	import { LoginForm } from '#lib/components/organisms/index.js';
	import { CenteredCard } from '#lib/components/templates/index.js';
</script>

<CenteredCard title="Sign in to arrd">
	<LoginForm next={page.url.searchParams.get('next')} />
</CenteredCard>
