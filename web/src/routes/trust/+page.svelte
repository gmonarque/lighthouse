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
		Trash2
	} from 'lucide-svelte';
	import { api } from '$lib/api/client';
	import type { TrustEntry, TrustSettings } from '$lib/api/client';
	import { addToast, truncateNpub, formatDateTime } from '$lib/stores/app';

	let whitelist: TrustEntry[] = [];
	let blacklist: TrustEntry[] = [];
	let trustSettings: TrustSettings | null = null;
	let activeTab: 'whitelist' | 'blacklist' = 'whitelist';

	// Add forms
	let showAddWhitelist = false;
	let showAddBlacklist = false;
	let newNpub = '';
	let newAlias = '';
	let newNotes = '';
	let newReason = '';

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
			const [wl, bl, settings] = await Promise.all([
				api.getWhitelist(),
				api.getBlacklist(),
				api.getTrustSettings()
			]);
			whitelist = wl;
			blacklist = bl;
			trustSettings = settings;
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
	<div class="flex gap-2 mb-4">
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
	</div>

	<!-- Whitelist Tab -->
	{#if activeTab === 'whitelist'}
		<div class="card">
			<div class="flex items-center justify-between mb-4">
				<h3 class="font-medium text-white">Trusted Users</h3>
				<button class="btn-primary" onclick={() => (showAddWhitelist = true)}>
					<UserPlus class="w-4 h-4" />
					Add User
				</button>
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
							<button
								class="btn-icon btn-ghost text-red-400"
								onclick={() => removeFromWhitelist(entry.npub)}
							>
								<Trash2 class="w-4 h-4" />
							</button>
						</div>
					{/each}
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
</div>

<!-- Add to Whitelist Modal -->
{#if showAddWhitelist}
	<div class="modal-backdrop" onclick={() => (showAddWhitelist = false)}></div>
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
	<div class="modal-backdrop" onclick={() => (showAddBlacklist = false)}></div>
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
