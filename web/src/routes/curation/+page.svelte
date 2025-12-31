<script lang="ts">
	import { onMount } from 'svelte';
	import {
		FileCheck,
		FileX,
		BookOpen,
		CheckCircle,
		XCircle,
		AlertTriangle,
		Info,
		X,
		ChevronDown,
		Filter,
		RefreshCw
	} from 'lucide-svelte';
	import { api } from '$lib/api/client';
	import type {
		Ruleset,
		Decision,
		DecisionStats,
		ReasonCode,
		ActiveRulesetsResponse
	} from '$lib/api/client';
	import { addToast, truncateNpub } from '$lib/stores/app';

	let activeTab: 'decisions' | 'rulesets' = 'decisions';
	let decisions: Decision[] = [];
	let rulesets: Ruleset[] = [];
	let activeRulesets: ActiveRulesetsResponse | null = null;
	let decisionStats: DecisionStats | null = null;
	let reasonCodes: ReasonCode[] = [];
	let loading = true;

	// Filters
	let filterDecision = '';
	let filterInfohash = '';

	// Modal
	let showImportRuleset = false;
	let importContent = '';
	let importSource = '';

	onMount(async () => {
		await loadData();
	});

	async function loadData() {
		loading = true;
		try {
			const [decisionsRes, rulesetsRes, activeRes, statsRes, codesRes] = await Promise.all([
				api.getDecisions({ limit: 50 }).catch(() => ({ decisions: [], total: 0 })),
				api.getRulesets().catch(() => []),
				api.getActiveRulesets().catch(() => null),
				api.getDecisionStats().catch(() => null),
				api.getReasonCodes().catch(() => [])
			]);

			decisions = decisionsRes.decisions || [];
			rulesets = rulesetsRes || [];
			activeRulesets = activeRes;
			decisionStats = statsRes;
			reasonCodes = codesRes;
		} catch (error) {
			console.error('Failed to load curation data:', error);
			addToast('error', 'Failed to load curation data');
		} finally {
			loading = false;
		}
	}

	async function importRuleset() {
		if (!importContent) return;

		try {
			await api.importRuleset(importContent, importSource || undefined);
			addToast('success', 'Ruleset imported successfully');
			showImportRuleset = false;
			importContent = '';
			importSource = '';
			await loadData();
		} catch (error: any) {
			addToast('error', error.message || 'Failed to import ruleset');
		}
	}

	async function activateRuleset(id: string) {
		try {
			await api.activateRuleset(id);
			addToast('success', 'Ruleset activated');
			await loadData();
		} catch (error) {
			addToast('error', 'Failed to activate ruleset');
		}
	}

	async function deactivateRuleset(id: string) {
		try {
			await api.deactivateRuleset(id);
			addToast('success', 'Ruleset deactivated');
			await loadData();
		} catch (error) {
			addToast('error', 'Failed to deactivate ruleset');
		}
	}

	async function deleteRuleset(id: string) {
		if (!confirm('Delete this ruleset? This action cannot be undone.')) return;

		try {
			await api.deleteRuleset(id);
			addToast('success', 'Ruleset deleted');
			await loadData();
		} catch (error) {
			addToast('error', 'Failed to delete ruleset');
		}
	}

	function getDecisionColor(decision: string): string {
		return decision === 'accept' ? 'text-green-400' : 'text-red-400';
	}

	function getReasonCodeCategory(code: string): string {
		const rc = reasonCodes.find(r => r.code === code);
		return rc?.category || 'unknown';
	}

	$: filteredDecisions = decisions.filter(d => {
		if (filterDecision && d.decision !== filterDecision) return false;
		if (filterInfohash && !d.target_infohash.toLowerCase().includes(filterInfohash.toLowerCase())) return false;
		return true;
	});
</script>

<div class="page-header">
	<h1 class="text-2xl font-bold text-white">Curation</h1>
	<p class="text-surface-400 mt-1">View verification decisions and manage rulesets</p>
</div>

<div class="page-content">
	<!-- Stats Cards -->
	{#if decisionStats}
		<div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
			<div class="card">
				<div class="flex items-center gap-3">
					<div class="w-10 h-10 rounded-lg bg-blue-900/30 flex items-center justify-center">
						<FileCheck class="w-5 h-5 text-blue-400" />
					</div>
					<div>
						<p class="text-2xl font-bold text-white">{decisionStats.total_decisions}</p>
						<p class="text-xs text-surface-400">Total Decisions</p>
					</div>
				</div>
			</div>
			<div class="card">
				<div class="flex items-center gap-3">
					<div class="w-10 h-10 rounded-lg bg-green-900/30 flex items-center justify-center">
						<CheckCircle class="w-5 h-5 text-green-400" />
					</div>
					<div>
						<p class="text-2xl font-bold text-white">{decisionStats.accept_count}</p>
						<p class="text-xs text-surface-400">Accepted</p>
					</div>
				</div>
			</div>
			<div class="card">
				<div class="flex items-center gap-3">
					<div class="w-10 h-10 rounded-lg bg-red-900/30 flex items-center justify-center">
						<XCircle class="w-5 h-5 text-red-400" />
					</div>
					<div>
						<p class="text-2xl font-bold text-white">{decisionStats.reject_count}</p>
						<p class="text-xs text-surface-400">Rejected</p>
					</div>
				</div>
			</div>
			<div class="card">
				<div class="flex items-center gap-3">
					<div class="w-10 h-10 rounded-lg bg-purple-900/30 flex items-center justify-center">
						<BookOpen class="w-5 h-5 text-purple-400" />
					</div>
					<div>
						<p class="text-2xl font-bold text-white">{rulesets.length}</p>
						<p class="text-xs text-surface-400">Rulesets</p>
					</div>
				</div>
			</div>
		</div>
	{/if}

	<!-- Tabs -->
	<div class="flex gap-2 mb-4">
		<button
			class="btn-secondary {activeTab === 'decisions' ? 'bg-primary-900/30 border-primary-500 text-primary-400' : ''}"
			onclick={() => (activeTab = 'decisions')}
		>
			<FileCheck class="w-4 h-4" />
			Decisions
		</button>
		<button
			class="btn-secondary {activeTab === 'rulesets' ? 'bg-purple-900/30 border-purple-500 text-purple-400' : ''}"
			onclick={() => (activeTab = 'rulesets')}
		>
			<BookOpen class="w-4 h-4" />
			Rulesets
		</button>
		<button class="btn-icon btn-ghost ml-auto" onclick={loadData} title="Refresh">
			<RefreshCw class="w-4 h-4 {loading ? 'animate-spin' : ''}" />
		</button>
	</div>

	<!-- Decisions Tab -->
	{#if activeTab === 'decisions'}
		<div class="card">
			<div class="flex items-center justify-between mb-4">
				<h3 class="font-medium text-white">Verification Decisions</h3>
				<div class="flex gap-2">
					<select
						class="input text-sm py-1.5"
						bind:value={filterDecision}
					>
						<option value="">All decisions</option>
						<option value="accept">Accept only</option>
						<option value="reject">Reject only</option>
					</select>
					<input
						type="text"
						placeholder="Filter by infohash..."
						class="input text-sm py-1.5"
						bind:value={filterInfohash}
					/>
				</div>
			</div>

			{#if loading}
				<div class="text-center py-8">
					<RefreshCw class="w-8 h-8 text-surface-400 mx-auto animate-spin" />
					<p class="text-surface-500 mt-2">Loading decisions...</p>
				</div>
			{:else if filteredDecisions.length > 0}
				<div class="overflow-x-auto">
					<table class="w-full">
						<thead>
							<tr class="text-left text-xs text-surface-400 border-b border-surface-700">
								<th class="pb-2 px-2">Decision</th>
								<th class="pb-2 px-2">Infohash</th>
								<th class="pb-2 px-2">Curator</th>
								<th class="pb-2 px-2">Reason Codes</th>
								<th class="pb-2 px-2">Date</th>
							</tr>
						</thead>
						<tbody>
							{#each filteredDecisions as decision}
								<tr class="border-b border-surface-800 hover:bg-surface-800/50">
									<td class="py-3 px-2">
										<span class="flex items-center gap-2 {getDecisionColor(decision.decision)}">
											{#if decision.decision === 'accept'}
												<CheckCircle class="w-4 h-4" />
											{:else}
												<XCircle class="w-4 h-4" />
											{/if}
											<span class="capitalize">{decision.decision}</span>
										</span>
									</td>
									<td class="py-3 px-2">
										<code class="text-xs text-surface-400 font-mono">
											{decision.target_infohash.slice(0, 12)}...
										</code>
									</td>
									<td class="py-3 px-2">
										<code class="text-xs text-surface-400 font-mono">
											{truncateNpub(decision.curator_pubkey)}
										</code>
									</td>
									<td class="py-3 px-2">
										<div class="flex flex-wrap gap-1">
											{#each decision.reason_codes as code}
												<span class="text-xs px-1.5 py-0.5 rounded bg-surface-700 text-surface-300">
													{code}
												</span>
											{/each}
										</div>
									</td>
									<td class="py-3 px-2 text-xs text-surface-400">
										{new Date(decision.created_at).toLocaleDateString()}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{:else}
				<div class="text-center py-8">
					<FileCheck class="w-12 h-12 text-surface-600 mx-auto mb-3" />
					<p class="text-surface-500">No decisions found</p>
					<p class="text-sm text-surface-600 mt-1">Verification decisions from trusted curators will appear here</p>
				</div>
			{/if}
		</div>

		<!-- Reason Codes Reference -->
		{#if reasonCodes.length > 0}
			<div class="card mt-4">
				<h3 class="font-medium text-white mb-4 flex items-center gap-2">
					<Info class="w-4 h-4 text-primary-400" />
					Reason Codes Reference
				</h3>
				<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-2">
					{#each reasonCodes as code}
						<div class="p-3 bg-surface-800 rounded-lg">
							<div class="flex items-center gap-2 mb-1">
								<code class="text-xs font-mono text-primary-400">{code.code}</code>
								<span class="text-xs px-1.5 py-0.5 rounded bg-surface-700 text-surface-400">
									{code.category}
								</span>
								{#if code.deterministic}
									<span class="text-xs px-1.5 py-0.5 rounded bg-yellow-900/30 text-yellow-400">
										deterministic
									</span>
								{/if}
							</div>
							<p class="text-xs text-surface-400">{code.description}</p>
						</div>
					{/each}
				</div>
			</div>
		{/if}
	{/if}

	<!-- Rulesets Tab -->
	{#if activeTab === 'rulesets'}
		<div class="card">
			<div class="flex items-center justify-between mb-4">
				<h3 class="font-medium text-white">Rulesets</h3>
				<button class="btn-primary" onclick={() => (showImportRuleset = true)}>
					<BookOpen class="w-4 h-4" />
					Import Ruleset
				</button>
			</div>

			<!-- Active Rulesets -->
			{#if activeRulesets && (activeRulesets.censoring || activeRulesets.semantic)}
				<div class="mb-6">
					<h4 class="text-sm font-medium text-surface-300 mb-3">Active Rulesets</h4>
					<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
						{#if activeRulesets.censoring}
							<div class="p-4 bg-red-900/10 border border-red-900/30 rounded-lg">
								<div class="flex items-center gap-2 mb-2">
									<AlertTriangle class="w-5 h-5 text-red-400" />
									<span class="font-medium text-red-300">Censoring Ruleset</span>
								</div>
								<p class="text-sm text-surface-400">v{activeRulesets.censoring.version}</p>
								<code class="text-xs text-surface-500 font-mono">{activeRulesets.censoring.hash.slice(0, 16)}...</code>
								<p class="text-xs text-surface-500 mt-1">{activeRulesets.censoring.rule_count} rules</p>
							</div>
						{/if}
						{#if activeRulesets.semantic}
							<div class="p-4 bg-blue-900/10 border border-blue-900/30 rounded-lg">
								<div class="flex items-center gap-2 mb-2">
									<BookOpen class="w-5 h-5 text-blue-400" />
									<span class="font-medium text-blue-300">Semantic Ruleset</span>
								</div>
								<p class="text-sm text-surface-400">v{activeRulesets.semantic.version}</p>
								<code class="text-xs text-surface-500 font-mono">{activeRulesets.semantic.hash.slice(0, 16)}...</code>
								<p class="text-xs text-surface-500 mt-1">{activeRulesets.semantic.rule_count} rules</p>
							</div>
						{/if}
					</div>
				</div>
			{/if}

			<!-- All Rulesets -->
			{#if rulesets.length > 0}
				<div class="space-y-2">
					{#each rulesets as ruleset}
						<div class="flex items-center justify-between p-4 bg-surface-800 rounded-lg">
							<div class="flex items-center gap-3">
								<div class="w-10 h-10 rounded-lg {ruleset.type === 'censoring' ? 'bg-red-900/30' : 'bg-blue-900/30'} flex items-center justify-center">
									{#if ruleset.type === 'censoring'}
										<AlertTriangle class="w-5 h-5 text-red-400" />
									{:else}
										<BookOpen class="w-5 h-5 text-blue-400" />
									{/if}
								</div>
								<div>
									<div class="flex items-center gap-2">
										<span class="font-medium text-surface-100">{ruleset.id}</span>
										<span class="text-xs px-1.5 py-0.5 rounded bg-surface-700 text-surface-400">
											{ruleset.type}
										</span>
										{#if ruleset.is_active}
											<span class="text-xs px-1.5 py-0.5 rounded bg-green-900/30 text-green-400">
												active
											</span>
										{/if}
									</div>
									<div class="flex items-center gap-2 mt-1">
										<span class="text-xs text-surface-500">v{ruleset.version}</span>
										<code class="text-xs text-surface-500 font-mono">{ruleset.hash.slice(0, 12)}...</code>
									</div>
								</div>
							</div>
							<div class="flex gap-2">
								{#if ruleset.is_active}
									<button
										class="btn-secondary text-sm"
										onclick={() => deactivateRuleset(ruleset.id)}
									>
										Deactivate
									</button>
								{:else}
									<button
										class="btn-primary text-sm"
										onclick={() => activateRuleset(ruleset.id)}
									>
										Activate
									</button>
									<button
										class="btn-icon btn-ghost text-red-400"
										onclick={() => deleteRuleset(ruleset.id)}
									>
										<X class="w-4 h-4" />
									</button>
								{/if}
							</div>
						</div>
					{/each}
				</div>
			{:else}
				<div class="text-center py-8">
					<BookOpen class="w-12 h-12 text-surface-600 mx-auto mb-3" />
					<p class="text-surface-500">No rulesets imported</p>
					<p class="text-sm text-surface-600 mt-1">Import a ruleset to enable content verification</p>
				</div>
			{/if}
		</div>
	{/if}
</div>

<!-- Import Ruleset Modal -->
{#if showImportRuleset}
	<div class="modal-backdrop" onclick={() => (showImportRuleset = false)} onkeydown={(e) => e.key === 'Escape' && (showImportRuleset = false)} role="button" tabindex="-1"></div>
	<div class="modal max-w-2xl">
		<div class="modal-header">
			<h2 class="text-lg font-semibold text-white">Import Ruleset</h2>
			<button class="btn-icon btn-ghost" onclick={() => (showImportRuleset = false)}>
				<X class="w-5 h-5" />
			</button>
		</div>
		<div class="modal-body space-y-4">
			<div class="p-3 bg-surface-800 rounded-lg text-sm text-surface-400">
				Paste the JSON content of the ruleset below. Rulesets define rules for content verification.
			</div>
			<div>
				<label class="label" for="ruleset-content">Ruleset JSON</label>
				<textarea
					id="ruleset-content"
					bind:value={importContent}
					placeholder="Paste ruleset JSON here..."
					class="input font-mono text-sm"
					rows="10"
				></textarea>
			</div>
			<div>
				<label class="label" for="ruleset-source">Source URL (optional)</label>
				<input
					id="ruleset-source"
					type="text"
					bind:value={importSource}
					placeholder="https://example.com/ruleset.json"
					class="input"
				/>
			</div>
		</div>
		<div class="modal-footer">
			<button class="btn-secondary" onclick={() => (showImportRuleset = false)}>Cancel</button>
			<button class="btn-primary" onclick={importRuleset} disabled={!importContent}>
				Import Ruleset
			</button>
		</div>
	</div>
{/if}
