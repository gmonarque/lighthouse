<script lang="ts">
	import { onMount } from 'svelte';
	import {
		Flag,
		AlertTriangle,
		CheckCircle,
		Clock,
		Eye,
		X,
		RefreshCw,
		Plus,
		MessageSquare
	} from 'lucide-svelte';
	import { api } from '$lib/api/client';
	import type { Report, ReportsResponse } from '$lib/api/client';
	import { addToast, truncateNpub } from '$lib/stores/app';

	let reports: Report[] = [];
	let pendingReports: Report[] = [];
	let loading = true;

	// Filters
	let filterStatus = '';
	let filterCategory = '';

	// Submit report form
	let showSubmitReport = false;
	let reportKind = 'report';
	let reportCategory = 'dmca';
	let reportTargetEventId = '';
	let reportTargetInfohash = '';
	let reportEvidence = '';
	let reportScope = '';
	let reportJurisdiction = '';

	const categories = [
		{ value: 'dmca', label: 'DMCA Takedown' },
		{ value: 'illegal', label: 'Illegal Content' },
		{ value: 'spam', label: 'Spam' },
		{ value: 'malware', label: 'Malware' },
		{ value: 'false_info', label: 'False Information' },
		{ value: 'duplicate', label: 'Duplicate' },
		{ value: 'other', label: 'Other' }
	];

	const statusColors: Record<string, string> = {
		pending: 'bg-yellow-900/30 text-yellow-400',
		acknowledged: 'bg-blue-900/30 text-blue-400',
		investigating: 'bg-purple-900/30 text-purple-400',
		resolved: 'bg-green-900/30 text-green-400',
		rejected: 'bg-red-900/30 text-red-400'
	};

	onMount(async () => {
		await loadData();
	});

	async function loadData() {
		loading = true;
		try {
			const [allReports, pending] = await Promise.all([
				api.getReports().catch(() => ({ reports: [], total: 0 })),
				api.getPendingReports().catch(() => ({ reports: [], total: 0 }))
			]);
			reports = allReports.reports || [];
			pendingReports = pending.reports || [];
		} catch (error) {
			console.error('Failed to load reports:', error);
			addToast('error', 'Failed to load reports');
		} finally {
			loading = false;
		}
	}

	async function submitReport() {
		if (!reportCategory) return;
		if (!reportTargetEventId && !reportTargetInfohash) {
			addToast('error', 'Please provide a target event ID or infohash');
			return;
		}

		try {
			await api.submitReport({
				kind: reportKind,
				category: reportCategory,
				target_event_id: reportTargetEventId || undefined,
				target_infohash: reportTargetInfohash || undefined,
				evidence: reportEvidence || undefined,
				scope: reportScope || undefined,
				jurisdiction: reportJurisdiction || undefined
			});
			addToast('success', 'Report submitted successfully');
			showSubmitReport = false;
			resetForm();
			await loadData();
		} catch (error: any) {
			addToast('error', error.message || 'Failed to submit report');
		}
	}

	function resetForm() {
		reportKind = 'report';
		reportCategory = 'dmca';
		reportTargetEventId = '';
		reportTargetInfohash = '';
		reportEvidence = '';
		reportScope = '';
		reportJurisdiction = '';
	}

	async function acknowledgeReport(id: string) {
		try {
			await api.acknowledgeReport(id);
			addToast('success', 'Report acknowledged');
			await loadData();
		} catch (error) {
			addToast('error', 'Failed to acknowledge report');
		}
	}

	async function updateReportStatus(id: string, status: string) {
		const resolution = status === 'resolved' ? prompt('Resolution notes:') : undefined;
		if (status === 'resolved' && resolution === null) return;

		try {
			await api.updateReport(id, status, resolution || undefined);
			addToast('success', 'Report status updated');
			await loadData();
		} catch (error) {
			addToast('error', 'Failed to update report status');
		}
	}

	$: filteredReports = reports.filter(r => {
		if (filterStatus && r.status !== filterStatus) return false;
		if (filterCategory && r.category !== filterCategory) return false;
		return true;
	});
</script>

<div class="page-header">
	<h1 class="text-2xl font-bold text-white">Reports & Appeals</h1>
	<p class="text-surface-400 mt-1">Submit and manage content reports and appeals</p>
</div>

<div class="page-content">
	<!-- Pending Reports Alert -->
	{#if pendingReports.length > 0}
		<div class="p-4 bg-yellow-900/20 border border-yellow-800 rounded-lg mb-6 flex items-center gap-3">
			<AlertTriangle class="w-5 h-5 text-yellow-400 flex-shrink-0" />
			<div class="flex-1">
				<p class="font-medium text-yellow-200">
					{pendingReports.length} pending report{pendingReports.length === 1 ? '' : 's'}
				</p>
				<p class="text-sm text-yellow-300/70">
					Reports awaiting review
				</p>
			</div>
		</div>
	{/if}

	<!-- Stats -->
	<div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
		<div class="card">
			<div class="flex items-center gap-3">
				<div class="w-10 h-10 rounded-lg bg-blue-900/30 flex items-center justify-center">
					<Flag class="w-5 h-5 text-blue-400" />
				</div>
				<div>
					<p class="text-2xl font-bold text-white">{reports.length}</p>
					<p class="text-xs text-surface-400">Total Reports</p>
				</div>
			</div>
		</div>
		<div class="card">
			<div class="flex items-center gap-3">
				<div class="w-10 h-10 rounded-lg bg-yellow-900/30 flex items-center justify-center">
					<Clock class="w-5 h-5 text-yellow-400" />
				</div>
				<div>
					<p class="text-2xl font-bold text-white">{pendingReports.length}</p>
					<p class="text-xs text-surface-400">Pending</p>
				</div>
			</div>
		</div>
		<div class="card">
			<div class="flex items-center gap-3">
				<div class="w-10 h-10 rounded-lg bg-green-900/30 flex items-center justify-center">
					<CheckCircle class="w-5 h-5 text-green-400" />
				</div>
				<div>
					<p class="text-2xl font-bold text-white">
						{reports.filter(r => r.status === 'resolved').length}
					</p>
					<p class="text-xs text-surface-400">Resolved</p>
				</div>
			</div>
		</div>
		<div class="card">
			<div class="flex items-center gap-3">
				<div class="w-10 h-10 rounded-lg bg-purple-900/30 flex items-center justify-center">
					<MessageSquare class="w-5 h-5 text-purple-400" />
				</div>
				<div>
					<p class="text-2xl font-bold text-white">
						{reports.filter(r => r.kind === 'appeal').length}
					</p>
					<p class="text-xs text-surface-400">Appeals</p>
				</div>
			</div>
		</div>
	</div>

	<!-- Actions -->
	<div class="flex items-center justify-between mb-4">
		<div class="flex gap-2">
			<select class="input text-sm py-1.5" bind:value={filterStatus}>
				<option value="">All statuses</option>
				<option value="pending">Pending</option>
				<option value="acknowledged">Acknowledged</option>
				<option value="investigating">Investigating</option>
				<option value="resolved">Resolved</option>
				<option value="rejected">Rejected</option>
			</select>
			<select class="input text-sm py-1.5" bind:value={filterCategory}>
				<option value="">All categories</option>
				{#each categories as cat}
					<option value={cat.value}>{cat.label}</option>
				{/each}
			</select>
		</div>
		<div class="flex gap-2">
			<button class="btn-icon btn-ghost" onclick={loadData} title="Refresh">
				<RefreshCw class="w-4 h-4 {loading ? 'animate-spin' : ''}" />
			</button>
			<button class="btn-primary" onclick={() => (showSubmitReport = true)}>
				<Plus class="w-4 h-4" />
				Submit Report
			</button>
		</div>
	</div>

	<!-- Reports List -->
	<div class="card">
		{#if loading}
			<div class="text-center py-8">
				<RefreshCw class="w-8 h-8 text-surface-400 mx-auto animate-spin" />
				<p class="text-surface-500 mt-2">Loading reports...</p>
			</div>
		{:else if filteredReports.length > 0}
			<div class="space-y-3">
				{#each filteredReports as report}
					<div class="p-4 bg-surface-800 rounded-lg">
						<div class="flex items-start justify-between">
							<div class="flex-1">
								<div class="flex items-center gap-2 mb-2">
									<span class="text-xs px-2 py-0.5 rounded {statusColors[report.status] || 'bg-surface-700 text-surface-400'}">
										{report.status}
									</span>
									<span class="text-xs px-2 py-0.5 rounded bg-surface-700 text-surface-300">
										{report.category}
									</span>
									<span class="text-xs px-2 py-0.5 rounded bg-surface-700 text-surface-400">
										{report.kind}
									</span>
								</div>
								<div class="text-sm text-surface-400 space-y-1">
									{#if report.target_infohash}
										<p>
											<span class="text-surface-500">Infohash:</span>
											<code class="font-mono text-xs">{report.target_infohash}</code>
										</p>
									{/if}
									{#if report.target_event_id}
										<p>
											<span class="text-surface-500">Event:</span>
											<code class="font-mono text-xs">{report.target_event_id.slice(0, 16)}...</code>
										</p>
									{/if}
									{#if report.evidence}
										<p class="text-surface-300 mt-2">{report.evidence}</p>
									{/if}
									{#if report.resolution}
										<p class="mt-2 p-2 bg-surface-700 rounded text-surface-300">
											<span class="text-surface-500">Resolution:</span> {report.resolution}
										</p>
									{/if}
								</div>
								<div class="flex items-center gap-3 mt-3 text-xs text-surface-500">
									<span>ID: {report.report_id}</span>
									<span>Created: {new Date(report.created_at).toLocaleDateString()}</span>
									{#if report.reporter_pubkey}
										<span>Reporter: {truncateNpub(report.reporter_pubkey)}</span>
									{/if}
								</div>
							</div>
							<div class="flex gap-2 ml-4">
								{#if report.status === 'pending'}
									<button
										class="btn-secondary text-sm"
										onclick={() => acknowledgeReport(report.id)}
									>
										<Eye class="w-4 h-4" />
										Acknowledge
									</button>
								{/if}
								{#if report.status === 'acknowledged' || report.status === 'investigating'}
									<button
										class="btn-primary text-sm"
										onclick={() => updateReportStatus(report.id, 'resolved')}
									>
										<CheckCircle class="w-4 h-4" />
										Resolve
									</button>
									<button
										class="btn-secondary text-sm"
										onclick={() => updateReportStatus(report.id, 'rejected')}
									>
										Reject
									</button>
								{/if}
							</div>
						</div>
					</div>
				{/each}
			</div>
		{:else}
			<div class="text-center py-8">
				<Flag class="w-12 h-12 text-surface-600 mx-auto mb-3" />
				<p class="text-surface-500">No reports found</p>
				<p class="text-sm text-surface-600 mt-1">Reports and appeals will appear here</p>
			</div>
		{/if}
	</div>
</div>

<!-- Submit Report Modal -->
{#if showSubmitReport}
	<div class="modal-backdrop" onclick={() => (showSubmitReport = false)} onkeydown={(e) => e.key === 'Escape' && (showSubmitReport = false)} role="button" tabindex="-1"></div>
	<div class="modal max-w-lg">
		<div class="modal-header">
			<h2 class="text-lg font-semibold text-white">Submit Report</h2>
			<button class="btn-icon btn-ghost" onclick={() => (showSubmitReport = false)}>
				<X class="w-5 h-5" />
			</button>
		</div>
		<div class="modal-body space-y-4">
			<fieldset>
				<legend class="label">Report Type</legend>
				<div class="flex gap-2">
					<label class="flex-1">
						<input type="radio" bind:group={reportKind} value="report" class="sr-only" />
						<div class="p-3 rounded-lg border cursor-pointer transition-colors {reportKind === 'report' ? 'border-primary-500 bg-primary-900/20' : 'border-surface-700 hover:border-surface-600'}">
							<p class="font-medium text-surface-100">Report</p>
							<p class="text-xs text-surface-400">Report content violation</p>
						</div>
					</label>
					<label class="flex-1">
						<input type="radio" bind:group={reportKind} value="appeal" class="sr-only" />
						<div class="p-3 rounded-lg border cursor-pointer transition-colors {reportKind === 'appeal' ? 'border-primary-500 bg-primary-900/20' : 'border-surface-700 hover:border-surface-600'}">
							<p class="font-medium text-surface-100">Appeal</p>
							<p class="text-xs text-surface-400">Contest a decision</p>
						</div>
					</label>
				</div>
			</fieldset>

			<div>
				<label class="label" for="report-category">Category</label>
				<select id="report-category" class="input" bind:value={reportCategory}>
					{#each categories as cat}
						<option value={cat.value}>{cat.label}</option>
					{/each}
				</select>
			</div>

			<div>
				<label class="label" for="report-infohash">Target Infohash</label>
				<input
					id="report-infohash"
					type="text"
					bind:value={reportTargetInfohash}
					placeholder="40-character hex infohash"
					class="input font-mono"
				/>
			</div>

			<div>
				<label class="label" for="report-event">Or Target Event ID</label>
				<input
					id="report-event"
					type="text"
					bind:value={reportTargetEventId}
					placeholder="Nostr event ID"
					class="input font-mono"
				/>
			</div>

			<div>
				<label class="label" for="report-evidence">Evidence / Details</label>
				<textarea
					id="report-evidence"
					bind:value={reportEvidence}
					placeholder="Provide details about the violation..."
					class="input"
					rows="3"
				></textarea>
			</div>

			<div class="grid grid-cols-2 gap-4">
				<div>
					<label class="label" for="report-scope">Scope (optional)</label>
					<input
						id="report-scope"
						type="text"
						bind:value={reportScope}
						placeholder="e.g., global, regional"
						class="input"
					/>
				</div>
				<div>
					<label class="label" for="report-jurisdiction">Jurisdiction (optional)</label>
					<input
						id="report-jurisdiction"
						type="text"
						bind:value={reportJurisdiction}
						placeholder="e.g., US, EU"
						class="input"
					/>
				</div>
			</div>
		</div>
		<div class="modal-footer">
			<button class="btn-secondary" onclick={() => (showSubmitReport = false)}>Cancel</button>
			<button class="btn-primary" onclick={submitReport} disabled={!reportCategory}>
				Submit Report
			</button>
		</div>
	</div>
{/if}
