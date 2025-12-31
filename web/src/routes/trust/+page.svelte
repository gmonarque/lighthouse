<script lang="ts">
	import { onMount } from 'svelte';
	import {
		Shield,
		UserPlus,
		UserMinus,
		Ban,
		Check,
		X,
		Info,
		Trash2,
		Users,
		Crown,
		Settings,
		Radio,
		Loader2
	} from 'lucide-svelte';
	import { api } from '$lib/api/client';
	import type { TrustEntry, TrustSettings, Curator, AggregationPolicy } from '$lib/api/client';
	import { addToast, truncateNpub, formatDateTime } from '$lib/stores/app';

	let whitelist: TrustEntry[] = [];
	let blacklist: TrustEntry[] = [];
	let curators: Curator[] = [];
	let aggregationPolicy: AggregationPolicy | null = null;
	let trustSettings: TrustSettings | null = null;
	let activeTab: 'whitelist' | 'blacklist' | 'curators' = 'whitelist';

	// Add forms
	let showAddWhitelist = false;
	let showAddBlacklist = false;
	let showAddCurator = false;
	let showAggregationSettings = false;
	let newNpub = '';
	let newAlias = '';
	let newNotes = '';
	let newReason = '';
	let newCuratorPubkey = '';
	let newCuratorAlias = '';
	let newCuratorWeight = 1;
	let aggregationMode = 'quorum';
	let quorumRequired = 1;
	let weightThreshold = 0;

	// Relay discovery state
	let discoveringRelays: Set<string> = new Set();
	let discoveringAllRelays = false;

	const trustLevels = [
		{ value: 0, label: 'Whitelist Only', description: 'Only show content from whitelisted users' },
		{ value: 1, label: 'Friends', description: 'Show content from your follows (recommended)' },
		{ value: 2, label: 'Extended Network', description: 'Show content from friends of friends (risky)' }
	];

	onMount(async () => {
		await loadData();
	});

	async function loadData() {
		try {
			const [wl, bl, settings, curatorsRes, aggPolicy] = await Promise.all([
				api.getWhitelist(),
				api.getBlacklist(),
				api.getTrustSettings(),
				api.getCurators().catch(() => ({ curators: [], total: 0 })),
				api.getAggregationPolicy().catch(() => null)
			]);
			whitelist = wl;
			blacklist = bl;
			trustSettings = settings;
			curators = curatorsRes.curators || [];
			aggregationPolicy = aggPolicy;
			if (aggPolicy) {
				aggregationMode = aggPolicy.mode;
				quorumRequired = aggPolicy.quorum_required;
				weightThreshold = aggPolicy.weight_threshold;
			}
		} catch (error) {
			console.error('Failed to load trust data:', error);
			addToast('error', 'Failed to load trust data');
		}
	}

	async function updateTrustDepth(depth: number) {
		try {
			await api.updateTrustSettings(depth);
			trustSettings = { depth };
			addToast('success', 'Trust settings updated');
		} catch (error) {
			addToast('error', 'Failed to update trust settings');
		}
	}

	async function addToWhitelist() {
		if (!newNpub) return;

		try {
			await api.addToWhitelist(newNpub, newAlias || undefined, newNotes || undefined);
			addToast('success', 'User added to whitelist');
			showAddWhitelist = false;
			newNpub = '';
			newAlias = '';
			newNotes = '';
			await loadData();
		} catch (error) {
			addToast('error', 'Failed to add to whitelist');
		}
	}

	async function removeFromWhitelist(npub: string) {
		if (!confirm('Remove this user from whitelist?')) return;

		try {
			await api.removeFromWhitelist(npub);
			addToast('success', 'User removed from whitelist');
			await loadData();
		} catch (error) {
			addToast('error', 'Failed to remove from whitelist');
		}
	}

	async function addToBlacklist() {
		if (!newNpub) return;

		try {
			await api.addToBlacklist(newNpub, newReason || undefined);
			addToast('success', 'User blocked and content removed');
			showAddBlacklist = false;
			newNpub = '';
			newReason = '';
			await loadData();
		} catch (error) {
			addToast('error', 'Failed to block user');
		}
	}

	async function removeFromBlacklist(npub: string) {
		if (!confirm('Unblock this user?')) return;

		try {
			await api.removeFromBlacklist(npub);
			addToast('success', 'User unblocked');
			await loadData();
		} catch (error) {
			addToast('error', 'Failed to unblock user');
		}
	}

	// Relay discovery functions
	async function discoverUserRelays(npub: string) {
		discoveringRelays = new Set([...discoveringRelays, npub]);
		try {
			const result = await api.discoverUserRelays(npub);
			if (result.relays_added > 0) {
				addToast('success', `Discovered ${result.relays_added} new relay(s) for this user`);
			} else {
				addToast('info', 'No new relays found for this user');
			}
		} catch (error) {
			addToast('error', 'Failed to discover relays');
		} finally {
			discoveringRelays = new Set([...discoveringRelays].filter(n => n !== npub));
		}
	}

	async function discoverAllRelays() {
		if (whitelist.length === 0) {
			addToast('info', 'No users in whitelist to discover relays for');
			return;
		}

		discoveringAllRelays = true;
		try {
			const result = await api.discoverAllTrustedRelays();
			if (result.relays_added > 0) {
				addToast('success', `Discovered ${result.relays_added} new relay(s) from ${result.users_processed} user(s)`);
			} else {
				addToast('info', `Checked ${result.users_processed} user(s), no new relays found`);
			}
		} catch (error) {
			addToast('error', 'Failed to discover relays');
		} finally {
			discoveringAllRelays = false;
		}
	}

	// Curator functions
	async function addCurator() {
		if (!newCuratorPubkey) return;

		try {
			await api.addCurator(newCuratorPubkey, newCuratorAlias || undefined, newCuratorWeight);
			addToast('success', 'Curator added');
			showAddCurator = false;
			newCuratorPubkey = '';
			newCuratorAlias = '';
			newCuratorWeight = 1;
			await loadData();
		} catch (error) {
			addToast('error', 'Failed to add curator');
		}
	}

	async function revokeCurator(pubkey: string) {
		const reason = prompt('Reason for revoking trust (optional):');
		if (reason === null) return; // Cancelled

		try {
			await api.revokeCurator(pubkey, reason || undefined);
			addToast('success', 'Curator trust revoked');
			await loadData();
		} catch (error) {
			addToast('error', 'Failed to revoke curator');
		}
	}

	async function updateAggregationPolicySettings() {
		try {
			await api.updateAggregationPolicy(aggregationMode, quorumRequired, weightThreshold);
			addToast('success', 'Aggregation policy updated');
			showAggregationSettings = false;
			await loadData();
		} catch (error) {
			addToast('error', 'Failed to update aggregation policy');
		}
	}

	const aggregationModes = [
		{ value: 'any', label: 'Any', description: 'Accept if any trusted curator approves' },
		{ value: 'quorum', label: 'Quorum', description: 'Require N curators to agree' },
		{ value: 'weighted', label: 'Weighted', description: 'Use curator weights for voting' },
		{ value: 'all', label: 'All', description: 'All curators must agree' }
	];
</script>

<div class="page-header">
	<h1 class="text-2xl font-bold text-white">Web of Trust</h1>
	<p class="text-surface-400 mt-1">Manage your trust network and content filtering</p>
</div>

<div class="page-content">
	<!-- Trust Depth Slider -->
	<div class="card mb-6">
		<div class="flex items-center gap-3 mb-4">
			<Shield class="w-5 h-5 text-primary-400" />
			<h2 class="text-lg font-semibold text-white">Trust Level</h2>
		</div>

		<div class="space-y-4">
			{#each trustLevels as level}
				<label
					class="flex items-start gap-3 p-3 rounded-lg border cursor-pointer transition-colors {trustSettings?.depth === level.value ? 'border-primary-500 bg-primary-900/20' : 'border-surface-700 hover:border-surface-600'}"
				>
					<input
						type="radio"
						name="trust-depth"
						value={level.value}
						checked={trustSettings?.depth === level.value}
						onchange={() => updateTrustDepth(level.value)}
						class="mt-1"
					/>
					<div>
						<p class="font-medium text-surface-100">{level.label}</p>
						<p class="text-sm text-surface-400">{level.description}</p>
					</div>
				</label>
			{/each}
		</div>

		<div class="mt-4 p-3 bg-surface-800 rounded-lg flex items-start gap-2">
			<Info class="w-4 h-4 text-primary-400 mt-0.5 flex-shrink-0" />
			<p class="text-sm text-surface-400">
				Trust depth determines whose content appears in your index. Higher levels show more content but increase risk of spam/malicious uploads.
			</p>
		</div>
	</div>

	<!-- Tabs -->
	<div class="flex gap-2 mb-4 flex-wrap">
		<button
			class="btn-secondary {activeTab === 'whitelist' ? 'bg-primary-900/30 border-primary-500 text-primary-400' : ''}"
			onclick={() => (activeTab = 'whitelist')}
		>
			<Check class="w-4 h-4" />
			Whitelist ({whitelist.length})
		</button>
		<button
			class="btn-secondary {activeTab === 'blacklist' ? 'bg-red-900/30 border-red-500 text-red-400' : ''}"
			onclick={() => (activeTab = 'blacklist')}
		>
			<Ban class="w-4 h-4" />
			Blacklist ({blacklist.length})
		</button>
		<button
			class="btn-secondary {activeTab === 'curators' ? 'bg-yellow-900/30 border-yellow-500 text-yellow-400' : ''}"
			onclick={() => (activeTab = 'curators')}
		>
			<Crown class="w-4 h-4" />
			Curators ({curators.length})
		</button>
	</div>

	<!-- Whitelist Tab -->
	{#if activeTab === 'whitelist'}
		<div class="card">
			<div class="flex items-center justify-between mb-4">
				<h3 class="font-medium text-white">Trusted Users</h3>
				<div class="flex gap-2">
					<button
						class="btn-secondary"
						onclick={discoverAllRelays}
						disabled={discoveringAllRelays || whitelist.length === 0}
						title="Discover relays for all whitelisted users"
					>
						{#if discoveringAllRelays}
							<Loader2 class="w-4 h-4 animate-spin" />
						{:else}
							<Radio class="w-4 h-4" />
						{/if}
						Discover All Relays
					</button>
					<button class="btn-primary" onclick={() => (showAddWhitelist = true)}>
						<UserPlus class="w-4 h-4" />
						Add User
					</button>
				</div>
			</div>

			{#if whitelist.length > 0}
				<div class="space-y-2">
					{#each whitelist as entry}
						<div class="flex items-center justify-between p-3 bg-surface-800 rounded-lg">
							<div>
								<div class="flex items-center gap-2">
									{#if entry.alias}
										<span class="font-medium text-surface-100">{entry.alias}</span>
									{/if}
									<code class="text-xs text-surface-400 font-mono">
										{truncateNpub(entry.npub)}
									</code>
								</div>
								{#if entry.notes}
									<p class="text-xs text-surface-500 mt-1">{entry.notes}</p>
								{/if}
							</div>
							<div class="flex gap-2">
								<button
									class="btn-icon btn-ghost text-primary-400"
									onclick={() => discoverUserRelays(entry.npub)}
									disabled={discoveringRelays.has(entry.npub)}
									title="Discover user's relays"
								>
									{#if discoveringRelays.has(entry.npub)}
										<Loader2 class="w-4 h-4 animate-spin" />
									{:else}
										<Radio class="w-4 h-4" />
									{/if}
								</button>
								<button
									class="btn-icon btn-ghost text-red-400"
									onclick={() => removeFromWhitelist(entry.npub)}
									title="Remove from whitelist"
								>
									<Trash2 class="w-4 h-4" />
								</button>
							</div>
						</div>
					{/each}
				</div>

				<!-- Info about relay discovery -->
				<div class="mt-4 p-3 bg-surface-800 rounded-lg flex items-start gap-2">
					<Info class="w-4 h-4 text-primary-400 mt-0.5 flex-shrink-0" />
					<p class="text-sm text-surface-400">
						Click <Radio class="w-3 h-3 inline" /> to discover which relays a user publishes to (NIP-65).
						This helps find more of their torrents. After discovering, run a resync from Settings.
					</p>
				</div>
			{:else}
				<p class="text-surface-500 text-center py-8">No users in whitelist</p>
			{/if}
		</div>
	{/if}

	<!-- Blacklist Tab -->
	{#if activeTab === 'blacklist'}
		<div class="card">
			<div class="flex items-center justify-between mb-4">
				<h3 class="font-medium text-white">Blocked Users</h3>
				<button class="btn-danger" onclick={() => (showAddBlacklist = true)}>
					<Ban class="w-4 h-4" />
					Block User
				</button>
			</div>

			{#if blacklist.length > 0}
				<div class="space-y-2">
					{#each blacklist as entry}
						<div class="flex items-center justify-between p-3 bg-surface-800 rounded-lg">
							<div>
								<code class="text-xs text-surface-400 font-mono">
									{truncateNpub(entry.npub)}
								</code>
								{#if entry.reason}
									<p class="text-xs text-surface-500 mt-1">Reason: {entry.reason}</p>
								{/if}
							</div>
							<button
								class="btn-icon btn-ghost text-green-400"
								onclick={() => removeFromBlacklist(entry.npub)}
							>
								<UserMinus class="w-4 h-4" />
							</button>
						</div>
					{/each}
				</div>
			{:else}
				<p class="text-surface-500 text-center py-8">No blocked users</p>
			{/if}
		</div>
	{/if}

	<!-- Curators Tab -->
	{#if activeTab === 'curators'}
		<div class="card mb-4">
			<div class="flex items-center justify-between mb-4">
				<div>
					<h3 class="font-medium text-white">Trusted Curators</h3>
					<p class="text-sm text-surface-400 mt-1">Curators whose verification decisions you trust for content filtering</p>
				</div>
				<div class="flex gap-2">
					<button class="btn-secondary" onclick={() => (showAggregationSettings = true)}>
						<Settings class="w-4 h-4" />
						Aggregation
					</button>
					<button class="btn-primary" onclick={() => (showAddCurator = true)}>
						<Crown class="w-4 h-4" />
						Add Curator
					</button>
				</div>
			</div>

			<!-- Aggregation Policy Info -->
			{#if aggregationPolicy}
				<div class="p-3 bg-surface-800 rounded-lg mb-4 flex items-center justify-between">
					<div>
						<span class="text-sm text-surface-400">Aggregation Mode:</span>
						<span class="text-sm font-medium text-surface-100 ml-2 capitalize">{aggregationPolicy.mode}</span>
						{#if aggregationPolicy.mode === 'quorum'}
							<span class="text-xs text-surface-500 ml-2">({aggregationPolicy.quorum_required} required)</span>
						{:else if aggregationPolicy.mode === 'weighted'}
							<span class="text-xs text-surface-500 ml-2">(threshold: {aggregationPolicy.weight_threshold})</span>
						{/if}
					</div>
				</div>
			{/if}

			{#if curators.length > 0}
				<div class="space-y-2">
					{#each curators as curator}
						<div class="flex items-center justify-between p-3 bg-surface-800 rounded-lg">
							<div class="flex items-center gap-3">
								<div class="w-10 h-10 rounded-full bg-yellow-900/30 flex items-center justify-center">
									<Crown class="w-5 h-5 text-yellow-400" />
								</div>
								<div>
									<div class="flex items-center gap-2">
										{#if curator.alias}
											<span class="font-medium text-surface-100">{curator.alias}</span>
										{/if}
										<code class="text-xs text-surface-400 font-mono">
											{truncateNpub(curator.pubkey)}
										</code>
									</div>
									<div class="flex items-center gap-2 mt-1">
										<span class="text-xs px-2 py-0.5 rounded bg-yellow-900/30 text-yellow-400">
											Weight: {curator.weight}
										</span>
										<span class="text-xs px-2 py-0.5 rounded {curator.status === 'active' ? 'bg-green-900/30 text-green-400' : 'bg-red-900/30 text-red-400'}">
											{curator.status}
										</span>
									</div>
								</div>
							</div>
							<button
								class="btn-icon btn-ghost text-red-400"
								onclick={() => revokeCurator(curator.pubkey)}
								title="Revoke trust"
							>
								<Trash2 class="w-4 h-4" />
							</button>
						</div>
					{/each}
				</div>
			{:else}
				<div class="text-center py-8">
					<Crown class="w-12 h-12 text-surface-600 mx-auto mb-3" />
					<p class="text-surface-500">No trusted curators</p>
					<p class="text-sm text-surface-600 mt-1">Add curators to use federated content verification</p>
				</div>
			{/if}
		</div>

		<!-- Info Box -->
		<div class="p-4 bg-yellow-900/10 border border-yellow-900/30 rounded-lg">
			<div class="flex gap-3">
				<Info class="w-5 h-5 text-yellow-400 flex-shrink-0 mt-0.5" />
				<div class="text-sm">
					<p class="text-yellow-200 font-medium">About Federated Curation</p>
					<p class="text-yellow-300/70 mt-1">
						Curators are trusted entities who verify torrent content. Their decisions help filter out spam,
						illegal content, and low-quality uploads. Configure the aggregation policy to control how
						multiple curator decisions are combined.
					</p>
				</div>
			</div>
		</div>
	{/if}
</div>

<!-- Add to Whitelist Modal -->
{#if showAddWhitelist}
	<div class="modal-backdrop" onclick={() => (showAddWhitelist = false)} onkeydown={(e) => e.key === 'Escape' && (showAddWhitelist = false)} role="button" tabindex="-1"></div>
	<div class="modal">
		<div class="modal-header">
			<h2 class="text-lg font-semibold text-white">Add to Whitelist</h2>
			<button class="btn-icon btn-ghost" onclick={() => (showAddWhitelist = false)}>
				<X class="w-5 h-5" />
			</button>
		</div>
		<div class="modal-body space-y-4">
			<div>
				<label class="label" for="npub">Nostr Public Key (npub)</label>
				<input
					id="npub"
					type="text"
					bind:value={newNpub}
					placeholder="npub1..."
					class="input font-mono"
				/>
			</div>
			<div>
				<label class="label" for="alias">Alias (optional)</label>
				<input
					id="alias"
					type="text"
					bind:value={newAlias}
					placeholder="Friendly name"
					class="input"
				/>
			</div>
			<div>
				<label class="label" for="notes">Notes (optional)</label>
				<textarea
					id="notes"
					bind:value={newNotes}
					placeholder="Why you trust this user..."
					class="input"
					rows="2"
				></textarea>
			</div>
		</div>
		<div class="modal-footer">
			<button class="btn-secondary" onclick={() => (showAddWhitelist = false)}>Cancel</button>
			<button class="btn-primary" onclick={addToWhitelist} disabled={!newNpub}>
				Add to Whitelist
			</button>
		</div>
	</div>
{/if}

<!-- Add to Blacklist Modal -->
{#if showAddBlacklist}
	<div class="modal-backdrop" onclick={() => (showAddBlacklist = false)} onkeydown={(e) => e.key === 'Escape' && (showAddBlacklist = false)} role="button" tabindex="-1"></div>
	<div class="modal">
		<div class="modal-header">
			<h2 class="text-lg font-semibold text-white">Block User</h2>
			<button class="btn-icon btn-ghost" onclick={() => (showAddBlacklist = false)}>
				<X class="w-5 h-5" />
			</button>
		</div>
		<div class="modal-body space-y-4">
			<div class="p-3 bg-red-900/20 border border-red-800 rounded-lg text-sm text-red-300">
				Blocking a user will immediately remove all their content from your index.
			</div>
			<div>
				<label class="label" for="block-npub">Nostr Public Key (npub)</label>
				<input
					id="block-npub"
					type="text"
					bind:value={newNpub}
					placeholder="npub1..."
					class="input font-mono"
				/>
			</div>
			<div>
				<label class="label" for="reason">Reason (optional)</label>
				<input
					id="reason"
					type="text"
					bind:value={newReason}
					placeholder="Why you're blocking this user..."
					class="input"
				/>
			</div>
		</div>
		<div class="modal-footer">
			<button class="btn-secondary" onclick={() => (showAddBlacklist = false)}>Cancel</button>
			<button class="btn-danger" onclick={addToBlacklist} disabled={!newNpub}>
				Block User
			</button>
		</div>
	</div>
{/if}

<!-- Add Curator Modal -->
{#if showAddCurator}
	<div class="modal-backdrop" onclick={() => (showAddCurator = false)} onkeydown={(e) => e.key === 'Escape' && (showAddCurator = false)} role="button" tabindex="-1"></div>
	<div class="modal">
		<div class="modal-header">
			<h2 class="text-lg font-semibold text-white">Add Trusted Curator</h2>
			<button class="btn-icon btn-ghost" onclick={() => (showAddCurator = false)}>
				<X class="w-5 h-5" />
			</button>
		</div>
		<div class="modal-body space-y-4">
			<div class="p-3 bg-yellow-900/20 border border-yellow-800 rounded-lg text-sm text-yellow-300">
				Curators are trusted entities whose content verification decisions will be used to filter your index.
			</div>
			<div>
				<label class="label" for="curator-pubkey">Curator Public Key</label>
				<input
					id="curator-pubkey"
					type="text"
					bind:value={newCuratorPubkey}
					placeholder="npub1... or hex pubkey"
					class="input font-mono"
				/>
			</div>
			<div>
				<label class="label" for="curator-alias">Alias (optional)</label>
				<input
					id="curator-alias"
					type="text"
					bind:value={newCuratorAlias}
					placeholder="Curator name"
					class="input"
				/>
			</div>
			<div>
				<label class="label" for="curator-weight">Weight</label>
				<input
					id="curator-weight"
					type="number"
					min="1"
					max="100"
					bind:value={newCuratorWeight}
					class="input"
				/>
				<p class="text-xs text-surface-500 mt-1">Higher weight = more influence in aggregated decisions</p>
			</div>
		</div>
		<div class="modal-footer">
			<button class="btn-secondary" onclick={() => (showAddCurator = false)}>Cancel</button>
			<button class="btn-primary" onclick={addCurator} disabled={!newCuratorPubkey}>
				Add Curator
			</button>
		</div>
	</div>
{/if}

<!-- Aggregation Settings Modal -->
{#if showAggregationSettings}
	<div class="modal-backdrop" onclick={() => (showAggregationSettings = false)} onkeydown={(e) => e.key === 'Escape' && (showAggregationSettings = false)} role="button" tabindex="-1"></div>
	<div class="modal">
		<div class="modal-header">
			<h2 class="text-lg font-semibold text-white">Aggregation Policy</h2>
			<button class="btn-icon btn-ghost" onclick={() => (showAggregationSettings = false)}>
				<X class="w-5 h-5" />
			</button>
		</div>
		<div class="modal-body space-y-4">
			<p class="text-sm text-surface-400">
				Configure how decisions from multiple curators are combined to determine content acceptance.
			</p>
			<fieldset>
				<legend class="label">Aggregation Mode</legend>
				<div class="space-y-2">
					{#each aggregationModes as mode}
						<label
							class="flex items-start gap-3 p-3 rounded-lg border cursor-pointer transition-colors {aggregationMode === mode.value ? 'border-primary-500 bg-primary-900/20' : 'border-surface-700 hover:border-surface-600'}"
						>
							<input
								type="radio"
								name="aggregation-mode"
								value={mode.value}
								bind:group={aggregationMode}
								class="mt-1"
							/>
							<div>
								<p class="font-medium text-surface-100">{mode.label}</p>
								<p class="text-sm text-surface-400">{mode.description}</p>
							</div>
						</label>
					{/each}
				</div>
			</fieldset>

			{#if aggregationMode === 'quorum'}
				<div>
					<label class="label" for="quorum">Quorum Required</label>
					<input
						id="quorum"
						type="number"
						min="1"
						bind:value={quorumRequired}
						class="input"
					/>
					<p class="text-xs text-surface-500 mt-1">Number of curators that must agree</p>
				</div>
			{/if}

			{#if aggregationMode === 'weighted'}
				<div>
					<label class="label" for="threshold">Weight Threshold</label>
					<input
						id="threshold"
						type="number"
						min="0"
						bind:value={weightThreshold}
						class="input"
					/>
					<p class="text-xs text-surface-500 mt-1">Minimum combined weight for acceptance</p>
				</div>
			{/if}
		</div>
		<div class="modal-footer">
			<button class="btn-secondary" onclick={() => (showAggregationSettings = false)}>Cancel</button>
			<button class="btn-primary" onclick={updateAggregationPolicySettings}>
				Save Policy
			</button>
		</div>
	</div>
{/if}
