<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import {
		Settings,
		Key,
		User,
		Database,
		Download,
		Upload,
		Play,
		Square,
		RefreshCw,
		Copy,
		Eye,
		EyeOff,
		Film,
		Filter,
		Plus,
		X
	} from 'lucide-svelte';
	import { api } from '$lib/api/client';
	import type { AppSettings, IndexerStatus } from '$lib/api/client';
	import { addToast, indexerStatus, formatBytes } from '$lib/stores/app';

	let settings: AppSettings | null = null;
	let showNsec = false;
	let tmdbApiKey = '';
	let omdbApiKey = '';
	let isGenerating = false;
	let importNsec = '';
	let showImportForm = false;
	let refreshInterval: ReturnType<typeof setInterval>;

	// Tag filter state
	let tagFilterEnabled = false;
	let tagFilter: string[] = [];
	let newTag = '';

	// Common tags from NIP-35
	const suggestedTags = ['movie', 'tv', 'music', 'games', 'software', 'books', 'xxx', '4k', 'hd', 'uhd'];

	onMount(async () => {
		await loadSettings();
		// Auto-refresh indexer status every 3 seconds
		refreshInterval = setInterval(refreshIndexerStatus, 3000);
	});

	onDestroy(() => {
		if (refreshInterval) {
			clearInterval(refreshInterval);
		}
	});

	async function refreshIndexerStatus() {
		try {
			const indexerData = await api.getIndexerStatus();
			indexerStatus.set(indexerData);
		} catch (error) {
			// Silent fail for background refresh
		}
	}

	async function loadSettings() {
		try {
			const [settingsData, indexerData] = await Promise.all([
				api.getSettings(),
				api.getIndexerStatus()
			]);
			settings = settingsData;
			indexerStatus.set(indexerData);

			// Load tag filter settings
			tagFilterEnabled = settingsData.indexer?.tag_filter_enabled ?? false;
			tagFilter = settingsData.indexer?.tag_filter ?? [];
		} catch (error) {
			console.error('Failed to load settings:', error);
			addToast('error', 'Failed to load settings');
		}
	}

	async function generateIdentity() {
		if (!confirm('Generate a new Nostr identity? This will replace your current identity.')) return;

		isGenerating = true;
		try {
			const result = await api.generateIdentity();
			addToast('success', 'New identity generated');
			await loadSettings();
		} catch (error) {
			addToast('error', 'Failed to generate identity');
		} finally {
			isGenerating = false;
		}
	}

	async function importIdentity() {
		if (!importNsec) return;

		try {
			await api.importIdentity(importNsec);
			addToast('success', 'Identity imported');
			showImportForm = false;
			importNsec = '';
			await loadSettings();
		} catch (error) {
			addToast('error', 'Failed to import identity. Check your nsec.');
		}
	}

	async function copyApiKey() {
		if (settings?.server.api_key) {
			navigator.clipboard.writeText(settings.server.api_key);
			addToast('success', 'API key copied to clipboard');
		}
	}

	async function copyTorznabUrl() {
		const url = `${window.location.origin}/api/torznab?apikey=YOUR_API_KEY`;
		navigator.clipboard.writeText(url);
		addToast('success', 'Torznab URL copied to clipboard');
	}

	async function updateEnrichmentKeys() {
		try {
			const updates: Record<string, string> = {};
			if (tmdbApiKey) updates['enrichment.tmdb_api_key'] = tmdbApiKey;
			if (omdbApiKey) updates['enrichment.omdb_api_key'] = omdbApiKey;

			if (Object.keys(updates).length > 0) {
				await api.updateSettings(updates);
				addToast('success', 'API keys updated');
				tmdbApiKey = '';
				omdbApiKey = '';
				await loadSettings();
			}
		} catch (error) {
			addToast('error', 'Failed to update API keys');
		}
	}

	async function startIndexer() {
		try {
			await api.startIndexer();
			addToast('success', 'Indexer started');
			await loadSettings();
		} catch (error) {
			addToast('error', 'Failed to start indexer');
		}
	}

	async function stopIndexer() {
		try {
			await api.stopIndexer();
			addToast('success', 'Indexer stopped');
			await loadSettings();
		} catch (error) {
			addToast('error', 'Failed to stop indexer');
		}
	}

	async function exportConfig() {
		try {
			const config = await api.exportConfig();
			const blob = new Blob([JSON.stringify(config, null, 2)], { type: 'application/json' });
			const url = URL.createObjectURL(blob);
			const a = document.createElement('a');
			a.href = url;
			a.download = 'lighthouse-config.json';
			a.click();
			URL.revokeObjectURL(url);
			addToast('success', 'Configuration exported');
		} catch (error) {
			addToast('error', 'Failed to export configuration');
		}
	}

	async function updateTagFilter() {
		try {
			await api.updateSettings({
				'indexer.tag_filter': tagFilter,
				'indexer.tag_filter_enabled': tagFilterEnabled
			});
			addToast('success', 'Tag filter settings saved');
		} catch (error) {
			addToast('error', 'Failed to save tag filter settings');
		}
	}

	function addTag(tag: string) {
		const normalizedTag = tag.toLowerCase().trim();
		if (normalizedTag && !tagFilter.includes(normalizedTag)) {
			tagFilter = [...tagFilter, normalizedTag];
		}
		newTag = '';
	}

	function removeTag(tag: string) {
		tagFilter = tagFilter.filter((t) => t !== tag);
	}

	function handleTagKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter') {
			event.preventDefault();
			addTag(newTag);
		}
	}
</script>

<div class="page-header">
	<h1 class="text-2xl font-bold text-white">Settings</h1>
	<p class="text-surface-400 mt-1">Configure your Lighthouse instance</p>
</div>

<div class="page-content space-y-6">
	<!-- Nostr Identity -->
	<div class="card">
		<div class="flex items-center gap-3 mb-4">
			<User class="w-5 h-5 text-primary-400" />
			<h2 class="text-lg font-semibold text-white">Nostr Identity</h2>
		</div>

		<div class="space-y-4">
			<div>
				<label class="label">Public Key (npub)</label>
				<div class="flex gap-2">
					<input
						type="text"
						value={settings?.nostr.identity.npub || 'Not configured'}
						readonly
						class="input font-mono text-sm flex-1"
					/>
					<button
						class="btn-secondary"
						onclick={() => navigator.clipboard.writeText(settings?.nostr.identity.npub || '')}
					>
						<Copy class="w-4 h-4" />
					</button>
				</div>
			</div>

			<div>
				<label class="label">Private Key (nsec)</label>
				<div class="flex gap-2">
					<input
						type={showNsec ? 'text' : 'password'}
						value={settings?.nostr.identity.nsec || 'Not configured'}
						readonly
						class="input font-mono text-sm flex-1"
					/>
					<button
						class="btn-secondary"
						onclick={() => (showNsec = !showNsec)}
					>
						{#if showNsec}
							<EyeOff class="w-4 h-4" />
						{:else}
							<Eye class="w-4 h-4" />
						{/if}
					</button>
				</div>
				<p class="text-xs text-red-400 mt-1">Never share your nsec with anyone!</p>
			</div>

			<div class="flex gap-2">
				<button class="btn-primary" onclick={generateIdentity} disabled={isGenerating}>
					<RefreshCw class="w-4 h-4 {isGenerating ? 'animate-spin' : ''}" />
					Generate New Identity
				</button>
				<button class="btn-secondary" onclick={() => (showImportForm = !showImportForm)}>
					<Upload class="w-4 h-4" />
					Import Existing
				</button>
			</div>

			{#if showImportForm}
				<div class="p-4 bg-surface-800 rounded-lg">
					<label class="label">Import nsec</label>
					<div class="flex gap-2">
						<input
							type="password"
							bind:value={importNsec}
							placeholder="nsec1..."
							class="input font-mono flex-1"
						/>
						<button class="btn-primary" onclick={importIdentity} disabled={!importNsec}>
							Import
						</button>
					</div>
				</div>
			{/if}
		</div>
	</div>

	<!-- Torznab API -->
	<div class="card">
		<div class="flex items-center gap-3 mb-4">
			<Key class="w-5 h-5 text-primary-400" />
			<h2 class="text-lg font-semibold text-white">Torznab API</h2>
		</div>

		<div class="space-y-4">
			<div>
				<label class="label">API Key</label>
				<div class="flex gap-2">
					<input
						type="text"
						value={settings?.server.api_key || 'Not set'}
						readonly
						class="input font-mono text-sm flex-1"
					/>
					<button class="btn-secondary" onclick={copyApiKey}>
						<Copy class="w-4 h-4" />
					</button>
				</div>
			</div>

			<div>
				<label class="label">Torznab URL (for Prowlarr/Sonarr/Radarr)</label>
				<div class="flex gap-2">
					<input
						type="text"
						value="{window.location.origin}/api/torznab"
						readonly
						class="input font-mono text-sm flex-1"
					/>
					<button class="btn-secondary" onclick={copyTorznabUrl}>
						<Copy class="w-4 h-4" />
					</button>
				</div>
			</div>
		</div>
	</div>

	<!-- Metadata Enrichment -->
	<div class="card">
		<div class="flex items-center gap-3 mb-4">
			<Film class="w-5 h-5 text-primary-400" />
			<h2 class="text-lg font-semibold text-white">Metadata Enrichment</h2>
		</div>

		<div class="space-y-4">
			<div class="flex items-center gap-2 text-sm">
				<span class="text-surface-400">Status:</span>
				{#if settings?.enrichment.enabled}
					<span class="badge badge-success">Enabled</span>
				{:else}
					<span class="badge badge-warning">Disabled</span>
				{/if}
			</div>

			<div>
				<label class="label">TMDB API Key</label>
				<div class="flex gap-2">
					<input
						type="password"
						bind:value={tmdbApiKey}
						placeholder={settings?.enrichment.tmdb_api_key ? '••• Configured' : 'Enter API key'}
						class="input flex-1"
					/>
				</div>
				<a href="https://www.themoviedb.org/settings/api" target="_blank" class="text-xs text-primary-400 hover:underline">
					Get a free TMDB API key &rarr;
				</a>
			</div>

			<div>
				<label class="label">OMDB API Key</label>
				<div class="flex gap-2">
					<input
						type="password"
						bind:value={omdbApiKey}
						placeholder={settings?.enrichment.omdb_api_key ? '••• Configured' : 'Enter API key'}
						class="input flex-1"
					/>
				</div>
				<a href="https://www.omdbapi.com/apikey.aspx" target="_blank" class="text-xs text-primary-400 hover:underline">
					Get a free OMDB API key &rarr;
				</a>
			</div>

			<button class="btn-primary" onclick={updateEnrichmentKeys} disabled={!tmdbApiKey && !omdbApiKey}>
				Save API Keys
			</button>
		</div>
	</div>

	<!-- Indexer Control -->
	<div class="card">
		<div class="flex items-center gap-3 mb-4">
			<Database class="w-5 h-5 text-primary-400" />
			<h2 class="text-lg font-semibold text-white">Indexer</h2>
		</div>

		<div class="space-y-4">
			<div class="flex items-center gap-4">
				<span class="text-surface-400">Status:</span>
				{#if $indexerStatus?.running}
					<span class="badge badge-success">Running</span>
				{:else}
					<span class="badge badge-danger">Stopped</span>
				{/if}
			</div>

			<div class="flex items-center gap-4 text-sm text-surface-400">
				<span>Torrents: {$indexerStatus?.total_torrents?.toLocaleString() ?? 0}</span>
				<span>Relays: {$indexerStatus?.connected_relays ?? 0}</span>
			</div>

			<div class="flex gap-2">
				{#if $indexerStatus?.running}
					<button class="btn-danger" onclick={stopIndexer}>
						<Square class="w-4 h-4" />
						Stop Indexer
					</button>
				{:else}
					<button class="btn-primary" onclick={startIndexer}>
						<Play class="w-4 h-4" />
						Start Indexer
					</button>
				{/if}
			</div>
		</div>
	</div>

	<!-- Tag Filter -->
	<div class="card">
		<div class="flex items-center gap-3 mb-4">
			<Filter class="w-5 h-5 text-primary-400" />
			<h2 class="text-lg font-semibold text-white">Content Tag Filter</h2>
		</div>

		<div class="space-y-4">
			<p class="text-sm text-surface-400">
				Filter which torrents to index based on NIP-35 content tags. When enabled, only torrents with at least one matching tag will be indexed.
			</p>

			<div class="flex items-center gap-3">
				<label class="flex items-center gap-2 cursor-pointer">
					<input
						type="checkbox"
						bind:checked={tagFilterEnabled}
						class="w-4 h-4 rounded border-surface-600 bg-surface-700 text-primary-500 focus:ring-primary-500"
					/>
					<span class="text-white">Enable tag filtering</span>
				</label>
			</div>

			{#if tagFilterEnabled}
				<div class="space-y-3">
					<!-- Current tags -->
					<div>
						<label class="label">Active Tags</label>
						<div class="flex flex-wrap gap-2 min-h-[2.5rem] p-2 bg-surface-800 rounded-lg">
							{#if tagFilter.length === 0}
								<span class="text-surface-500 text-sm">No tags configured - all torrents will be indexed</span>
							{:else}
								{#each tagFilter as tag}
									<span class="inline-flex items-center gap-1 px-2 py-1 bg-primary-500/20 text-primary-400 rounded-md text-sm">
										{tag}
										<button
											onclick={() => removeTag(tag)}
											class="hover:text-red-400 transition-colors"
											aria-label="Remove tag"
										>
											<X class="w-3 h-3" />
										</button>
									</span>
								{/each}
							{/if}
						</div>
					</div>

					<!-- Add new tag -->
					<div>
						<label class="label">Add Tag</label>
						<div class="flex gap-2">
							<input
								type="text"
								bind:value={newTag}
								onkeydown={handleTagKeydown}
								placeholder="Enter tag name..."
								class="input flex-1"
							/>
							<button
								class="btn-secondary"
								onclick={() => addTag(newTag)}
								disabled={!newTag.trim()}
							>
								<Plus class="w-4 h-4" />
								Add
							</button>
						</div>
					</div>

					<!-- Suggested tags -->
					<div>
						<label class="label">Suggested Tags</label>
						<div class="flex flex-wrap gap-2">
							{#each suggestedTags.filter((t) => !tagFilter.includes(t)) as tag}
								<button
									onclick={() => addTag(tag)}
									class="px-2 py-1 bg-surface-700 hover:bg-surface-600 text-surface-300 rounded-md text-sm transition-colors"
								>
									+ {tag}
								</button>
							{/each}
						</div>
					</div>
				</div>
			{/if}

			<button class="btn-primary" onclick={updateTagFilter}>
				Save Tag Filter Settings
			</button>
		</div>
	</div>

	<!-- Backup -->
	<div class="card">
		<div class="flex items-center gap-3 mb-4">
			<Download class="w-5 h-5 text-primary-400" />
			<h2 class="text-lg font-semibold text-white">Backup & Restore</h2>
		</div>

		<div class="flex gap-2">
			<button class="btn-secondary" onclick={exportConfig}>
				<Download class="w-4 h-4" />
				Export Configuration
			</button>
		</div>
	</div>
</div>
