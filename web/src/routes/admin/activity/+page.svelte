<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import {
		Activity,
		RefreshCw,
		Filter,
		Clock,
		Database,
		Radio,
		Shield,
		FileCheck,
		Flag,
		Settings,
		ChevronLeft,
		ChevronRight
	} from 'lucide-svelte';
	import { api } from '$lib/api/client';
	import type { Activity as ActivityType } from '$lib/api/client';
	import { addToast } from '$lib/stores/app';

	let activities: ActivityType[] = [];
	let loading = true;
	let autoRefresh = true;
	let refreshInterval: ReturnType<typeof setInterval>;
	let selectedType = '';
	let offset = 0;
	const limit = 50;

	const eventTypes = [
		{ value: '', label: 'All Events' },
		{ value: 'torrent_indexed', label: 'Torrents Indexed' },
		{ value: 'relay_connected', label: 'Relay Connected' },
		{ value: 'relay_disconnected', label: 'Relay Disconnected' },
		{ value: 'decision_made', label: 'Curation Decisions' },
		{ value: 'report_submitted', label: 'Reports' },
		{ value: 'settings_changed', label: 'Settings Changed' },
		{ value: 'error', label: 'Errors' }
	];

	onMount(async () => {
		await loadActivity();
		if (autoRefresh) {
			refreshInterval = setInterval(loadActivity, 5000);
		}
	});

	onDestroy(() => {
		if (refreshInterval) clearInterval(refreshInterval);
	});

	async function loadActivity() {
		try {
			loading = activities.length === 0;
			const data = await api.getActivity(limit, offset, selectedType || undefined);
			activities = data || [];
		} catch (error) {
			if (activities.length === 0) {
				addToast('error', 'Failed to load activity');
			}
		} finally {
			loading = false;
		}
	}

	function toggleAutoRefresh() {
		autoRefresh = !autoRefresh;
		if (autoRefresh) {
			refreshInterval = setInterval(loadActivity, 5000);
		} else if (refreshInterval) {
			clearInterval(refreshInterval);
		}
	}

	function getEventIcon(type: string) {
		switch (type) {
			case 'torrent_indexed':
				return Database;
			case 'relay_connected':
			case 'relay_disconnected':
				return Radio;
			case 'decision_made':
				return FileCheck;
			case 'report_submitted':
				return Flag;
			case 'settings_changed':
				return Settings;
			case 'error':
				return Shield;
			default:
				return Activity;
		}
	}

	function getEventColor(type: string): string {
		switch (type) {
			case 'torrent_indexed':
				return 'text-green-400';
			case 'relay_connected':
				return 'text-blue-400';
			case 'relay_disconnected':
				return 'text-yellow-400';
			case 'decision_made':
				return 'text-purple-400';
			case 'report_submitted':
				return 'text-orange-400';
			case 'error':
				return 'text-red-400';
			default:
				return 'text-surface-400';
		}
	}

	function formatTime(dateStr: string): string {
		const date = new Date(dateStr);
		const now = new Date();
		const diff = now.getTime() - date.getTime();
		const minutes = Math.floor(diff / 60000);
		const hours = Math.floor(diff / 3600000);
		const days = Math.floor(diff / 86400000);

		if (minutes < 1) return 'Just now';
		if (minutes < 60) return `${minutes}m ago`;
		if (hours < 24) return `${hours}h ago`;
		if (days < 7) return `${days}d ago`;
		return date.toLocaleDateString();
	}

	function prevPage() {
		if (offset >= limit) {
			offset -= limit;
			loadActivity();
		}
	}

	function nextPage() {
		if (activities.length === limit) {
			offset += limit;
			loadActivity();
		}
	}

	$: if (selectedType !== undefined) {
		offset = 0;
		loadActivity();
	}
</script>

<div class="page-header">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold text-white">Activity Log</h1>
			<p class="text-surface-400 mt-1">Monitor system activity and events</p>
		</div>
		<div class="flex items-center gap-2">
			<button
				class="btn-secondary {autoRefresh ? 'bg-green-500/20 text-green-400' : ''}"
				onclick={toggleAutoRefresh}
				title={autoRefresh ? 'Auto-refresh ON' : 'Auto-refresh OFF'}
			>
				<RefreshCw class="w-4 h-4 {autoRefresh ? 'animate-spin-slow' : ''}" />
				{autoRefresh ? 'Live' : 'Paused'}
			</button>
			<button class="btn-secondary" onclick={loadActivity}>
				<RefreshCw class="w-4 h-4" />
				Refresh
			</button>
		</div>
	</div>
</div>

<div class="page-content space-y-6">
	<!-- Filters -->
	<div class="card">
		<div class="flex items-center gap-4">
			<Filter class="w-5 h-5 text-surface-400" />
			<select bind:value={selectedType} class="input max-w-xs">
				{#each eventTypes as type}
					<option value={type.value}>{type.label}</option>
				{/each}
			</select>
		</div>
	</div>

	<!-- Activity List -->
	<div class="card">
		<div class="flex items-center gap-3 mb-4">
			<Activity class="w-5 h-5 text-primary-400" />
			<h2 class="text-lg font-semibold text-white">Recent Activity</h2>
			{#if autoRefresh}
				<span class="flex items-center gap-1 text-xs text-green-400">
					<span class="w-2 h-2 rounded-full bg-green-400 animate-pulse"></span>
					Live
				</span>
			{/if}
		</div>

		{#if loading}
			<div class="text-center py-8 text-surface-400">Loading activity...</div>
		{:else if activities.length === 0}
			<div class="text-center py-8 text-surface-400">
				<Activity class="w-12 h-12 mx-auto mb-3 opacity-50" />
				<p>No activity recorded yet</p>
			</div>
		{:else}
			<div class="space-y-2">
				{#each activities as activity}
					<div class="flex items-start gap-3 p-3 bg-surface-800 rounded-lg hover:bg-surface-750 transition-colors">
						<div class="p-2 bg-surface-700 rounded {getEventColor(activity.event_type)}">
							<svelte:component this={getEventIcon(activity.event_type)} class="w-4 h-4" />
						</div>
						<div class="flex-1 min-w-0">
							<div class="flex items-center gap-2">
								<span class="font-medium text-white capitalize">
									{activity.event_type.replace(/_/g, ' ')}
								</span>
								<span class="text-xs text-surface-500">
									{formatTime(activity.created_at)}
								</span>
							</div>
							{#if activity.details}
								<p class="text-sm text-surface-400 mt-0.5 truncate">{activity.details}</p>
							{/if}
						</div>
						<div class="text-xs text-surface-500 flex items-center gap-1">
							<Clock class="w-3 h-3" />
							{new Date(activity.created_at).toLocaleTimeString()}
						</div>
					</div>
				{/each}
			</div>

			<!-- Pagination -->
			<div class="flex items-center justify-between mt-4 pt-4 border-t border-surface-700">
				<button
					class="btn-secondary"
					onclick={prevPage}
					disabled={offset === 0}
				>
					<ChevronLeft class="w-4 h-4" />
					Previous
				</button>
				<span class="text-sm text-surface-400">
					Showing {offset + 1} - {offset + activities.length}
				</span>
				<button
					class="btn-secondary"
					onclick={nextPage}
					disabled={activities.length < limit}
				>
					Next
					<ChevronRight class="w-4 h-4" />
				</button>
			</div>
		{/if}
	</div>
</div>

<style>
	.animate-spin-slow {
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
