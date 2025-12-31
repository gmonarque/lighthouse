<script lang="ts">
	import { onMount } from 'svelte';
	import {
		Clock,
		CheckCircle,
		XCircle,
		AlertTriangle,
		TrendingUp,
		RefreshCw,
		Flag,
		Shield,
		Activity
	} from 'lucide-svelte';
	import { api } from '$lib/api/client';
	import type { SLAStatus, SLAHistory } from '$lib/api/client';
	import { addToast } from '$lib/stores/app';

	let status: SLAStatus | null = null;
	let history: SLAHistory | null = null;
	let loading = true;

	onMount(async () => {
		await Promise.all([loadStatus(), loadHistory()]);
	});

	async function loadStatus() {
		try {
			loading = true;
			status = await api.getSLAStatus();
		} catch (error) {
			addToast('error', 'Failed to load SLA status');
		} finally {
			loading = false;
		}
	}

	async function loadHistory() {
		try {
			history = await api.getSLAHistory();
		} catch (error) {
			console.error('Failed to load SLA history:', error);
		}
	}

	function getStatusColor(s: string): string {
		switch (s) {
			case 'healthy':
				return 'text-green-400';
			case 'warning':
				return 'text-yellow-400';
			case 'degraded':
				return 'text-red-400';
			default:
				return 'text-surface-400';
		}
	}

	function getStatusIcon(s: string) {
		switch (s) {
			case 'healthy':
				return CheckCircle;
			case 'warning':
				return AlertTriangle;
			case 'degraded':
				return XCircle;
			default:
				return Activity;
		}
	}

	function formatHours(hours: number): string {
		if (hours < 1) return `${Math.round(hours * 60)}m`;
		if (hours < 24) return `${hours.toFixed(1)}h`;
		return `${(hours / 24).toFixed(1)}d`;
	}
</script>

<div class="page-header">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold text-white">SLA Monitoring</h1>
			<p class="text-surface-400 mt-1">Service Level Agreement compliance and metrics</p>
		</div>
		<button class="btn-secondary" onclick={() => Promise.all([loadStatus(), loadHistory()])}>
			<RefreshCw class="w-4 h-4" />
			Refresh
		</button>
	</div>
</div>

<div class="page-content space-y-6">
	{#if loading}
		<div class="text-center py-12 text-surface-400">Loading SLA status...</div>
	{:else if status}
		<!-- Overall Status -->
		<div class="card {status.overall_compliant ? 'border-green-500/30' : 'border-yellow-500/30'} border">
			<div class="flex items-center gap-4">
				<div class="p-4 rounded-full {status.overall_compliant ? 'bg-green-500/20' : 'bg-yellow-500/20'}">
					<svelte:component
						this={getStatusIcon(status.status)}
						class="w-8 h-8 {getStatusColor(status.status)}"
					/>
				</div>
				<div>
					<h2 class="text-xl font-bold {getStatusColor(status.status)} capitalize">
						{status.status}
					</h2>
					<p class="text-surface-400">
						{status.overall_compliant
							? 'All SLA requirements are being met'
							: 'Some SLA requirements need attention'}
					</p>
				</div>
			</div>
		</div>

		<!-- Report SLA -->
		<div class="grid md:grid-cols-2 gap-6">
			<div class="card">
				<div class="flex items-center gap-3 mb-4">
					<Flag class="w-5 h-5 text-primary-400" />
					<h2 class="text-lg font-semibold text-white">Report Acknowledgment SLA</h2>
				</div>
				<div class="space-y-4">
					<div class="flex justify-between items-center">
						<span class="text-surface-400">Target</span>
						<span class="text-white font-medium">{status.thresholds.acknowledgment_hours}h</span>
					</div>
					<div class="flex justify-between items-center">
						<span class="text-surface-400">Compliance Rate</span>
						<span class="text-xl font-bold {status.reports.sla_compliance_rate >= 95 ? 'text-green-400' : status.reports.sla_compliance_rate >= 80 ? 'text-yellow-400' : 'text-red-400'}">
							{status.reports.sla_compliance_rate.toFixed(1)}%
						</span>
					</div>
					<div class="h-3 bg-surface-800 rounded-full overflow-hidden">
						<div
							class="h-full transition-all duration-500 {status.reports.sla_compliance_rate >= 95 ? 'bg-green-500' : status.reports.sla_compliance_rate >= 80 ? 'bg-yellow-500' : 'bg-red-500'}"
							style="width: {Math.min(status.reports.sla_compliance_rate, 100)}%"
						></div>
					</div>
					<div class="grid grid-cols-2 gap-4 pt-2">
						<div class="p-3 bg-surface-800 rounded text-center">
							<div class="text-2xl font-bold text-white">{status.reports.pending}</div>
							<div class="text-xs text-surface-400">Pending</div>
						</div>
						<div class="p-3 bg-surface-800 rounded text-center">
							<div class="text-2xl font-bold {status.reports.past_sla > 0 ? 'text-red-400' : 'text-green-400'}">
								{status.reports.past_sla}
							</div>
							<div class="text-xs text-surface-400">Past SLA</div>
						</div>
					</div>
					<div class="flex justify-between items-center p-3 bg-surface-800 rounded">
						<span class="text-surface-400">Avg. Acknowledgment Time</span>
						<span class="font-medium text-white">{formatHours(status.reports.avg_ack_time_hours)}</span>
					</div>
				</div>
			</div>

			<div class="card">
				<div class="flex items-center gap-3 mb-4">
					<CheckCircle class="w-5 h-5 text-primary-400" />
					<h2 class="text-lg font-semibold text-white">Resolution Metrics</h2>
				</div>
				<div class="space-y-4">
					<div class="flex justify-between items-center">
						<span class="text-surface-400">Target Resolution Time</span>
						<span class="text-white font-medium">{status.thresholds.resolution_hours}h</span>
					</div>
					<div class="flex justify-between items-center">
						<span class="text-surface-400">Total Resolved</span>
						<span class="text-xl font-bold text-white">{status.resolution.total_resolved}</span>
					</div>
					<div class="flex justify-between items-center p-3 bg-surface-800 rounded">
						<span class="text-surface-400">Avg. Resolution Time</span>
						<span class="font-medium text-white">{formatHours(status.resolution.avg_resolution_time_hours)}</span>
					</div>

					{#if status.resolution.by_status}
						<div class="pt-2">
							<span class="text-sm text-surface-400">By Status</span>
							<div class="grid grid-cols-2 gap-2 mt-2">
								{#each Object.entries(status.resolution.by_status) as [s, count]}
									<div class="p-2 bg-surface-800 rounded text-center">
										<div class="font-bold text-white">{count}</div>
										<div class="text-xs text-surface-400 capitalize">{s}</div>
									</div>
								{/each}
							</div>
						</div>
					{/if}
				</div>
			</div>
		</div>

		<!-- System Health -->
		<div class="card">
			<div class="flex items-center gap-3 mb-4">
				<Shield class="w-5 h-5 text-primary-400" />
				<h2 class="text-lg font-semibold text-white">System Health</h2>
			</div>
			<div class="grid md:grid-cols-3 gap-4">
				<div class="p-4 bg-surface-800 rounded text-center">
					<div class="text-3xl font-bold {status.system.uptime_percent >= 99 ? 'text-green-400' : 'text-yellow-400'}">
						{status.system.uptime_percent.toFixed(1)}%
					</div>
					<div class="text-sm text-surface-400">Uptime</div>
				</div>
				<div class="p-4 bg-surface-800 rounded text-center">
					<div class="text-3xl font-bold {status.system.recent_errors === 0 ? 'text-green-400' : status.system.recent_errors < 10 ? 'text-yellow-400' : 'text-red-400'}">
						{status.system.recent_errors}
					</div>
					<div class="text-sm text-surface-400">Errors (24h)</div>
				</div>
				<div class="p-4 bg-surface-800 rounded text-center">
					<Clock class="w-6 h-6 text-surface-400 mx-auto mb-1" />
					<div class="text-sm text-surface-400">
						Last Check: {new Date(status.system.last_check).toLocaleTimeString()}
					</div>
				</div>
			</div>
		</div>

		<!-- Historical Compliance -->
		{#if history && history.history.length > 0}
			<div class="card">
				<div class="flex items-center gap-3 mb-4">
					<TrendingUp class="w-5 h-5 text-primary-400" />
					<h2 class="text-lg font-semibold text-white">Compliance History ({history.period})</h2>
				</div>
				<div class="overflow-x-auto">
					<table class="w-full text-sm">
						<thead>
							<tr class="text-left text-surface-400 border-b border-surface-700">
								<th class="pb-2 font-medium">Date</th>
								<th class="pb-2 font-medium text-right">Reports</th>
								<th class="pb-2 font-medium text-right">Within SLA</th>
								<th class="pb-2 font-medium text-right">Compliance</th>
							</tr>
						</thead>
						<tbody>
							{#each history.history.slice(-14) as day}
								<tr class="border-b border-surface-800">
									<td class="py-2 text-white">{day.date}</td>
									<td class="py-2 text-right text-surface-300">{day.total_reports}</td>
									<td class="py-2 text-right text-surface-300">{day.within_sla}</td>
									<td class="py-2 text-right">
										<span class="{day.compliance_rate >= 95 ? 'text-green-400' : day.compliance_rate >= 80 ? 'text-yellow-400' : 'text-red-400'}">
											{day.compliance_rate.toFixed(1)}%
										</span>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			</div>
		{/if}

		<!-- SLA Thresholds Info -->
		<div class="card bg-surface-800/50">
			<h3 class="text-sm font-medium text-surface-300 mb-2">SLA Thresholds</h3>
			<ul class="text-sm text-surface-400 space-y-1">
				<li>Report Acknowledgment: within {status.thresholds.acknowledgment_hours} hours</li>
				<li>Report Resolution: within {status.thresholds.resolution_hours} hours ({status.thresholds.resolution_hours / 24} days)</li>
				<li>System Uptime: {status.thresholds.uptime_percent}% minimum</li>
			</ul>
		</div>
	{/if}
</div>
