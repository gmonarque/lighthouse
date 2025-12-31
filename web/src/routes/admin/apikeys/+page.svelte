<script lang="ts">
	import { onMount } from 'svelte';
	import {
		Key,
		Plus,
		Trash2,
		Copy,
		Eye,
		EyeOff,
		ToggleLeft,
		ToggleRight,
		Shield,
		Clock,
		CheckCircle,
		XCircle
	} from 'lucide-svelte';
	import { api } from '$lib/api/client';
	import type { APIKeyDetail, PermissionInfo } from '$lib/api/client';
	import { addToast, formatBytes } from '$lib/stores/app';

	let keys: APIKeyDetail[] = [];
	let permissions: PermissionInfo[] = [];
	let loading = true;
	let showCreateForm = false;
	let newKeyVisible: string | null = null;

	// Create form state
	let newKey = {
		name: '',
		permissions: ['read', 'torznab'] as string[],
		rate_limit: 0,
		expires_in: 0,
		notes: ''
	};

	onMount(async () => {
		await Promise.all([loadKeys(), loadPermissions()]);
	});

	async function loadKeys() {
		try {
			loading = true;
			const response = await api.getAPIKeys();
			keys = response.keys || [];
		} catch (error) {
			addToast('error', 'Failed to load API keys');
		} finally {
			loading = false;
		}
	}

	async function loadPermissions() {
		try {
			permissions = await api.getAvailablePermissions();
		} catch (error) {
			console.error('Failed to load permissions:', error);
		}
	}

	async function createKey() {
		if (!newKey.name) {
			addToast('error', 'Name is required');
			return;
		}

		try {
			const response = await api.createAPIKey({
				name: newKey.name,
				permissions: newKey.permissions,
				rate_limit: newKey.rate_limit || undefined,
				expires_in: newKey.expires_in || undefined,
				notes: newKey.notes || undefined
			});

			newKeyVisible = response.key;
			addToast('success', 'API key created. Copy it now - it won\'t be shown again!');

			// Reset form
			newKey = {
				name: '',
				permissions: ['read', 'torznab'],
				rate_limit: 0,
				expires_in: 0,
				notes: ''
			};

			await loadKeys();
		} catch (error) {
			addToast('error', 'Failed to create API key');
		}
	}

	async function deleteKey(id: string, name: string) {
		if (!confirm(`Delete API key "${name}"? This action cannot be undone.`)) return;

		try {
			await api.deleteAPIKey(id);
			addToast('success', 'API key deleted');
			await loadKeys();
		} catch (error) {
			addToast('error', 'Failed to delete API key');
		}
	}

	async function toggleKey(key: APIKeyDetail) {
		try {
			if (key.enabled) {
				await api.disableAPIKey(key.id);
				addToast('success', 'API key disabled');
			} else {
				await api.enableAPIKey(key.id);
				addToast('success', 'API key enabled');
			}
			await loadKeys();
		} catch (error) {
			addToast('error', 'Failed to toggle API key');
		}
	}

	function copyToClipboard(text: string) {
		navigator.clipboard.writeText(text);
		addToast('success', 'Copied to clipboard');
	}

	function togglePermission(perm: string) {
		if (newKey.permissions.includes(perm)) {
			newKey.permissions = newKey.permissions.filter((p) => p !== perm);
		} else {
			newKey.permissions = [...newKey.permissions, perm];
		}
	}

	function formatDate(dateStr: string | undefined): string {
		if (!dateStr) return 'Never';
		return new Date(dateStr).toLocaleString();
	}

	function isExpired(dateStr: string | undefined): boolean {
		if (!dateStr) return false;
		return new Date(dateStr) < new Date();
	}
</script>

<div class="page-header">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold text-white">API Keys</h1>
			<p class="text-surface-400 mt-1">Manage API keys for multi-user access</p>
		</div>
		<button class="btn-primary" onclick={() => (showCreateForm = !showCreateForm)}>
			<Plus class="w-4 h-4" />
			Create Key
		</button>
	</div>
</div>

<div class="page-content space-y-6">
	<!-- New Key Display (only shown after creation) -->
	{#if newKeyVisible}
		<div class="card bg-green-900/20 border border-green-500/30">
			<div class="flex items-start gap-3">
				<CheckCircle class="w-6 h-6 text-green-400 flex-shrink-0 mt-1" />
				<div class="flex-1">
					<h3 class="text-lg font-semibold text-green-400">API Key Created</h3>
					<p class="text-surface-300 text-sm mt-1">
						Copy this key now. It will not be shown again.
					</p>
					<div class="flex items-center gap-2 mt-3">
						<code class="flex-1 p-3 bg-surface-800 rounded font-mono text-sm text-white break-all">
							{newKeyVisible}
						</code>
						<button class="btn-secondary" onclick={() => copyToClipboard(newKeyVisible!)}>
							<Copy class="w-4 h-4" />
						</button>
					</div>
					<button
						class="text-sm text-surface-400 hover:text-white mt-2"
						onclick={() => (newKeyVisible = null)}
					>
						Dismiss
					</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- Create Form -->
	{#if showCreateForm}
		<div class="card">
			<div class="flex items-center gap-3 mb-4">
				<Key class="w-5 h-5 text-primary-400" />
				<h2 class="text-lg font-semibold text-white">Create New API Key</h2>
			</div>

			<div class="space-y-4">
				<div>
					<label class="label" for="key-name">Name</label>
					<input
						id="key-name"
						type="text"
						bind:value={newKey.name}
						placeholder="e.g., Prowlarr, Sonarr, Mobile App"
						class="input"
					/>
				</div>

				<div>
					<span class="label">Permissions</span>
					<div class="grid grid-cols-2 md:grid-cols-4 gap-2">
						{#each permissions as perm}
							<button
								class="p-2 rounded border text-left text-sm transition-colors {newKey.permissions.includes(perm.id)
									? 'bg-primary-500/20 border-primary-500 text-primary-400'
									: 'bg-surface-800 border-surface-700 text-surface-400 hover:border-surface-600'}"
								onclick={() => togglePermission(perm.id)}
							>
								<div class="font-medium">{perm.name}</div>
								<div class="text-xs opacity-70">{perm.description}</div>
							</button>
						{/each}
					</div>
				</div>

				<div class="grid grid-cols-2 gap-4">
					<div>
						<label class="label" for="rate-limit">Rate Limit (req/min, 0 = default)</label>
						<input
							id="rate-limit"
							type="number"
							bind:value={newKey.rate_limit}
							min="0"
							class="input"
						/>
					</div>
					<div>
						<label class="label" for="expires-in">Expires In (days, 0 = never)</label>
						<input
							id="expires-in"
							type="number"
							bind:value={newKey.expires_in}
							min="0"
							class="input"
						/>
					</div>
				</div>

				<div>
					<label class="label" for="notes">Notes (optional)</label>
					<textarea
						id="notes"
						bind:value={newKey.notes}
						placeholder="Additional notes about this key..."
						rows="2"
						class="input"
					></textarea>
				</div>

				<div class="flex gap-2">
					<button class="btn-primary" onclick={createKey}>
						<Plus class="w-4 h-4" />
						Create Key
					</button>
					<button class="btn-secondary" onclick={() => (showCreateForm = false)}>Cancel</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- Keys List -->
	<div class="card">
		<div class="flex items-center gap-3 mb-4">
			<Shield class="w-5 h-5 text-primary-400" />
			<h2 class="text-lg font-semibold text-white">Active Keys ({keys.length})</h2>
		</div>

		{#if loading}
			<div class="text-center py-8 text-surface-400">Loading...</div>
		{:else if keys.length === 0}
			<div class="text-center py-8 text-surface-400">
				<Key class="w-12 h-12 mx-auto mb-3 opacity-50" />
				<p>No API keys created yet</p>
				<p class="text-sm mt-1">Create a key to enable multi-user access</p>
			</div>
		{:else}
			<div class="space-y-3">
				{#each keys as key}
					<div
						class="p-4 bg-surface-800 rounded-lg border {key.enabled
							? 'border-surface-700'
							: 'border-red-500/30 opacity-60'}"
					>
						<div class="flex items-start justify-between gap-4">
							<div class="flex-1 min-w-0">
								<div class="flex items-center gap-2">
									<h3 class="font-medium text-white">{key.name}</h3>
									{#if !key.enabled}
										<span class="badge badge-danger">Disabled</span>
									{:else if isExpired(key.expires_at)}
										<span class="badge badge-warning">Expired</span>
									{:else}
										<span class="badge badge-success">Active</span>
									{/if}
								</div>
								<div class="text-sm text-surface-400 mt-1">
									<code class="bg-surface-700 px-1 rounded">{key.key_prefix}...</code>
								</div>
								<div class="flex flex-wrap gap-1 mt-2">
									{#each key.permissions as perm}
										<span class="px-2 py-0.5 bg-surface-700 text-surface-300 rounded text-xs">
											{perm}
										</span>
									{/each}
								</div>
								<div class="flex gap-4 text-xs text-surface-500 mt-2">
									<span>Created: {formatDate(key.created_at)}</span>
									<span>Last used: {formatDate(key.last_used_at)}</span>
									{#if key.expires_at}
										<span class={isExpired(key.expires_at) ? 'text-red-400' : ''}>
											Expires: {formatDate(key.expires_at)}
										</span>
									{/if}
								</div>
								{#if key.notes}
									<p class="text-xs text-surface-500 mt-1 italic">{key.notes}</p>
								{/if}
							</div>
							<div class="flex items-center gap-2">
								<button
									class="p-2 hover:bg-surface-700 rounded transition-colors"
									onclick={() => toggleKey(key)}
									title={key.enabled ? 'Disable' : 'Enable'}
								>
									{#if key.enabled}
										<ToggleRight class="w-5 h-5 text-green-400" />
									{:else}
										<ToggleLeft class="w-5 h-5 text-surface-400" />
									{/if}
								</button>
								<button
									class="p-2 hover:bg-red-500/20 rounded transition-colors text-red-400"
									onclick={() => deleteKey(key.id, key.name)}
									title="Delete"
								>
									<Trash2 class="w-5 h-5" />
								</button>
							</div>
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</div>

	<!-- Usage Info -->
	<div class="card bg-surface-800/50">
		<h3 class="text-sm font-medium text-surface-300 mb-2">Using API Keys</h3>
		<ul class="text-sm text-surface-400 space-y-1">
			<li>Include the key in the <code class="bg-surface-700 px-1 rounded">X-API-Key</code> header</li>
			<li>Or use the <code class="bg-surface-700 px-1 rounded">apikey</code> query parameter</li>
			<li>For Torznab: <code class="bg-surface-700 px-1 rounded">/api/torznab?apikey=YOUR_KEY</code></li>
		</ul>
	</div>
</div>
