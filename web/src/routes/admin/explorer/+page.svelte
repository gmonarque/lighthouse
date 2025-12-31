<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import {
		Compass,
		RefreshCw,
		Database,
		Radio,
		Users,
		Clock,
		TrendingUp,
		AlertTriangle,
		HardDrive,
		Activity
	} from 'lucide-svelte';
	import { api } from '$lib/api/client';
	import type { ExplorerStats } from '$lib/api/client';
	import { addToast, formatBytes } from '$lib/stores/app';

	let stats: ExplorerStats | null = null;
	let loading = true;
	let autoRefresh = true;
	let refreshInterval: ReturnType<typeof setInterval>;

	onMount(async () => {
		await loadStats();
		if (autoRefresh) {
			refreshInterval = setInterval(loadStats, 10000);
		}
	});

	onDestroy(() => {
		if (refreshInterval) clearInterval(refreshInterval);
	});

	async function loadStats() {
		try {
			loading = stats === null;
			stats = await api.getExplorerStats();
		} catch (error) {
			if (!stats) {
				addToast('error', 'Failed to load explorer stats');
			}
		} finally {
			loading = false;
		}
	}

	function toggleAutoRefresh() {
		autoRefresh = !autoRefresh;
		if (autoRefresh) {
			refreshInterval = setInterval(loadStats, 10000);
		} else if (refreshInterval) {
			clearInterval(refreshInterval);
		}
	}

	function formatNumber(n: number): string {
		if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M';
		if (n >= 1000) return (n / 1000).toFixed(1) + 'K';
		return n.toString();
	}

	// Computed values for charts
	$: maxHourlyCount = stats?.hourly_activity
		? Math.max(...stats.hourly_activity.map((h) => h.count), 1)
		: 1;
	$: totalEventTypes = stats?.event_types
		? stats.event_types.reduce((sum, e) => sum + e.count, 0)
		: 0;
</script>

<div class="page-header">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold text-white">Explorer Stats</h1>
			<p class="text-surface-400 mt-1">Monitor indexer and event discovery performance</p>
		</div>
		<div class="flex items-center gap-2">
			<button
				class="btn-secondary {autoRefresh ? 'bg-green-500/20 text-green-400' : ''}"
				onclick={toggleAutoRefresh}
			>
				<RefreshCw class="w-4 h-4 {autoRefresh ? 'animate-spin-slow' : ''}" />
				{autoRefresh ? 'Live' : 'Paused'}
			</button>
			<button class="btn-secondary" onclick={loadStats}>
				<RefreshCw class="w-4 h-4" />
				Refresh
			</button>
		</div>
	</div>
</div>

<div class="page-content space-y-6">
	{#if loading}
		<div class="text-center py-12 text-surface-400">Loading stats...</div>
	{:else if stats}
		<!-- Key Metrics -->
		<div class="grid grid-cols-2 md:grid-cols-4 gap-4">
			<div class="card text-center">
				<Database class="w-8 h-8 text-primary-400 mx-auto mb-2" />
				<div class="text-2xl font-bold text-white">{formatNumber(stats.total_torrents)}</div>
				<div class="text-sm text-surface-400">Total Torrents</div>
			</div>
			<div class="card text-center">
				<Radio class="w-8 h-8 text-green-400 mx-auto mb-2" />
				<div class="text-2xl font-bold text-white">{stats.connected_relays}</div>
				<div class="text-sm text-surface-400">Connected Relays</div>
			</div>
			<div class="card text-center">
				<Users class="w-8 h-8 text-blue-400 mx-auto mb-2" />
				<div class="text-2xl font-bold text-white">{formatNumber(stats.unique_uploaders)}</div>
				<div class="text-sm text-surface-400">Unique Uploaders</div>
			</div>
			<div class="card text-center">
				<HardDrive class="w-8 h-8 text-purple-400 mx-auto mb-2" />
				<div class="text-2xl font-bold text-white">{formatBytes(stats.database_size)}</div>
				<div class="text-sm text-surface-400">Database Size</div>
			</div>
		</div>

		<!-- Activity Stats -->
		<div class="grid md:grid-cols-2 gap-6">
			<div class="card">
				<div class="flex items-center gap-3 mb-4">
					<TrendingUp class="w-5 h-5 text-primary-400" />
					<h2 class="text-lg font-semibold text-white">Event Discovery</h2>
				</div>
				<div class="space-y-4">
					<div class="flex justify-between items-center p-3 bg-surface-800 rounded">
						<span class="text-surface-400">Total Events Discovered</span>
						<span class="text-xl font-bold text-white">{formatNumber(stats.events_discovered)}</span>
					</div>
					<div class="flex justify-between items-center p-3 bg-surface-800 rounded">
						<span class="text-surface-400">Events (Last Hour)</span>
						<span class="text-xl font-bold text-green-400">{formatNumber(stats.events_last_hour)}</span>
					</div>
					<div class="flex justify-between items-center p-3 bg-surface-800 rounded">
						<span class="text-surface-400">Events (Last 24h)</span>
						<span class="text-xl font-bold text-blue-400">{formatNumber(stats.events_last_24h)}</span>
					</div>
				</div>
			</div>

			<div class="card">
				<div class="flex items-center gap-3 mb-4">
					<Activity class="w-5 h-5 text-primary-400" />
					<h2 class="text-lg font-semibold text-white">Queue Status</h2>
				</div>
				<div class="space-y-4">
					<div class="flex justify-between items-center p-3 bg-surface-800 rounded">
						<span class="text-surface-400">Queue Length</span>
						<span class="text-xl font-bold text-white">{stats.queue_length}</span>
					</div>
					<div class="flex justify-between items-center p-3 bg-surface-800 rounded">
						<span class="text-surface-400">Events Dropped</span>
						<span class="text-xl font-bold {stats.events_dropped > 0 ? 'text-red-400' : 'text-green-400'}">
							{stats.events_dropped}
						</span>
					</div>
					{#if stats.events_dropped > 0}
						<div class="flex items-center gap-2 p-3 bg-yellow-500/10 border border-yellow-500/30 rounded">
							<AlertTriangle class="w-4 h-4 text-yellow-400" />
							<span class="text-sm text-yellow-400">
								Events are being dropped. Consider increasing queue size.
							</span>
						</div>
					{/if}
				</div>
			</div>
		</div>

		<!-- Event Types Breakdown -->
		{#if stats.event_types && stats.event_types.length > 0}
			<div class="card">
				<div class="flex items-center gap-3 mb-4">
					<Compass class="w-5 h-5 text-primary-400" />
					<h2 class="text-lg font-semibold text-white">Event Types (Last 7 Days)</h2>
				</div>
				<div class="space-y-2">
					{#each stats.event_types as eventType}
						{@const percentage = totalEventTypes > 0 ? (eventType.count / totalEventTypes) * 100 : 0}
						<div class="flex items-center gap-3">
							<div class="w-32 text-sm text-surface-400 truncate capitalize">
								{eventType.type.replace(/_/g, ' ')}
							</div>
							<div class="flex-1 h-6 bg-surface-800 rounded overflow-hidden">
								<div
									class="h-full bg-primary-500 transition-all duration-500"
									style="width: {percentage}%"
								></div>
							</div>
							<div class="w-20 text-right text-sm text-surface-300">
								{formatNumber(eventType.count)}
							</div>
						</div>
					{/each}
				</div>
			</div>
		{/if}

		<!-- Hourly Activity Chart -->
		{#if stats.hourly_activity && stats.hourly_activity.length > 0}
			<div class="card">
				<div class="flex items-center gap-3 mb-4">
					<Clock class="w-5 h-5 text-primary-400" />
					<h2 class="text-lg font-semibold text-white">Hourly Activity (Last 24h)</h2>
				</div>
				<div class="flex items-end gap-1 h-32">
					{#each stats.hourly_activity as hour}
						{@const height = (hour.count / maxHourlyCount) * 100}
						<div
							class="flex-1 bg-primary-500/50 hover:bg-primary-500 rounded-t transition-colors cursor-pointer"
							style="height: {height}%"
							title="{hour.hour}: {hour.count} events"
						></div>
					{/each}
				</div>
				<div class="flex justify-between text-xs text-surface-500 mt-2">
					<span>24h ago</span>
					<span>Now</span>
				</div>
			</div>
		{/if}
	{/if}
</div>

<style>
	:global(.animate-spin-slow) {
		animation: spin 2s linear infinite;
	}
	@keyframes spin {
		from {
			transform: rotate(0deg);
		}
		to {
			transform: rotate(360deg);
		}
	}
</style>
