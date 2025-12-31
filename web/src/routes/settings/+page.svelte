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
		X,
		Radio,
		Globe,
		Users,
		Shield
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

	// Relay server state
	let relayEnabled = false;
	let relayListen = '0.0.0.0:9998';
	let relayMode: 'public' | 'community' = 'community';
	let relayRequireCuration = true;
	let relaySyncWith: string[] = [];
	let relayEnableDiscovery = false;
	let newSyncRelay = '';

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

			// Load relay server settings
			relayEnabled = settingsData.relay?.enabled ?? false;
			relayListen = settingsData.relay?.listen ?? '0.0.0.0:9998';
			relayMode = settingsData.relay?.mode ?? 'community';
			relayRequireCuration = settingsData.relay?.require_curation ?? true;
			relaySyncWith = settingsData.relay?.sync_with ?? [];
			relayEnableDiscovery = settingsData.relay?.enable_discovery ?? false;
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

	let resyncDays = 30;

	async function resyncIndexer() {
		try {
			await api.resyncIndexer(resyncDays);
			const timeMsg = resyncDays === 0 ? 'all time' : `last ${resyncDays} days`;
			addToast('success', `Fetching historical torrents (${timeMsg})... Only trusted uploaders will be indexed.`);
		} catch (error) {
			addToast('error', 'Failed to start resync');
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

	async function updateRelaySettings() {
		try {
			await api.updateSettings({
				'relay.enabled': relayEnabled,
				'relay.listen': relayListen,
				'relay.mode': relayMode,
				'relay.require_curation': relayRequireCuration,
				'relay.sync_with': relaySyncWith,
				'relay.enable_discovery': relayEnableDiscovery
			});
			addToast('success', 'Relay server settings saved. Restart required for changes to take effect.');
		} catch (error) {
			addToast('error', 'Failed to save relay server settings');
		}
	}

	function addSyncRelay(url: string) {
		const normalizedUrl = url.trim();
		if (normalizedUrl && !relaySyncWith.includes(normalizedUrl)) {
			relaySyncWith = [...relaySyncWith, normalizedUrl];
		}
		newSyncRelay = '';
	}

	function removeSyncRelay(url: string) {
		relaySyncWith = relaySyncWith.filter((r) => r !== url);
	}

	function handleSyncRelayKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter') {
			event.preventDefault();
			addSyncRelay(newSyncRelay);
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
				<label class="label" for="settings-npub">Public Key (npub)</label>
				<div class="flex gap-2">
					<input
						id="settings-npub"
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
				<label class="label" for="settings-nsec">Private Key (nsec)</label>
				<div class="flex gap-2">
					<input
						id="settings-nsec"
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
					<label class="label" for="import-nsec">Import nsec</label>
					<div class="flex gap-2">
						<input
							id="import-nsec"
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
				<label class="label" for="settings-api-key">API Key</label>
				<div class="flex gap-2">
					<input
						id="settings-api-key"
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
				<label class="label" for="settings-torznab-url">Torznab URL (for Prowlarr/Sonarr/Radarr)</label>
				<div class="flex gap-2">
					<input
						id="settings-torznab-url"
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
				<label class="label" for="settings-tmdb-key">TMDB API Key</label>
				<div class="flex gap-2">
					<input
						id="settings-tmdb-key"
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
				<label class="label" for="settings-omdb-key">OMDB API Key</label>
				<div class="flex gap-2">
					<input
						id="settings-omdb-key"
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
					<div class="flex items-center gap-2">
						<select bind:value={resyncDays} class="input w-28 text-sm">
							<option value={7}>7 days</option>
							<option value={14}>14 days</option>
							<option value={30}>30 days</option>
							<option value={90}>90 days</option>
							<option value={180}>180 days</option>
							<option value={365}>1 year</option>
							<option value={0}>No limit</option>
						</select>
						<button class="btn-secondary" onclick={resyncIndexer}>
							<RefreshCw class="w-4 h-4" />
							Resync
						</button>
					</div>
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
						<span class="label">Active Tags</span>
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
						<label class="label" for="settings-add-tag">Add Tag</label>
						<div class="flex gap-2">
							<input
								id="settings-add-tag"
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
						<span class="label">Suggested Tags</span>
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

	<!-- Relay Server -->
	<div class="card">
		<div class="flex items-center gap-3 mb-4">
			<Radio class="w-5 h-5 text-primary-400" />
			<h2 class="text-lg font-semibold text-white">Nostr Relay Server</h2>
		</div>

		<div class="space-y-4">
			<p class="text-sm text-surface-400">
				Run your own Nostr relay server to share torrent metadata with others. This allows your instance to act as a relay for NIP-35 torrent events.
			</p>

			<div class="flex items-center gap-3">
				<label class="flex items-center gap-2 cursor-pointer">
					<input
						type="checkbox"
						bind:checked={relayEnabled}
						class="w-4 h-4 rounded border-surface-600 bg-surface-700 text-primary-500 focus:ring-primary-500"
					/>
					<span class="text-white">Enable Relay Server</span>
				</label>
			</div>

			{#if relayEnabled}
				<div class="space-y-4 p-4 bg-surface-800 rounded-lg">
					<!-- Listen Address -->
					<div>
						<label class="label" for="relay-listen">Listen Address</label>
						<input
							id="relay-listen"
							type="text"
							bind:value={relayListen}
							placeholder="0.0.0.0:9998"
							class="input font-mono"
						/>
						<p class="text-xs text-surface-500 mt-1">WebSocket server address (host:port)</p>
					</div>

					<!-- Mode -->
					<div>
						<span class="label">Relay Mode</span>
						<div class="flex gap-4 mt-2">
							<label class="flex items-center gap-2 cursor-pointer">
								<input
									type="radio"
									bind:group={relayMode}
									value="community"
									class="w-4 h-4 border-surface-600 bg-surface-700 text-primary-500 focus:ring-primary-500"
								/>
								<span class="flex items-center gap-1 text-white">
									<Users class="w-4 h-4 text-surface-400" />
									Community
								</span>
							</label>
							<label class="flex items-center gap-2 cursor-pointer">
								<input
									type="radio"
									bind:group={relayMode}
									value="public"
									class="w-4 h-4 border-surface-600 bg-surface-700 text-primary-500 focus:ring-primary-500"
								/>
								<span class="flex items-center gap-1 text-white">
									<Globe class="w-4 h-4 text-surface-400" />
									Public
								</span>
							</label>
						</div>
						<p class="text-xs text-surface-500 mt-1">
							{relayMode === 'community' ? 'Only accept events from trusted users' : 'Accept events from anyone'}
						</p>
					</div>

					<!-- Require Curation -->
					<div class="flex items-center gap-3">
						<label class="flex items-center gap-2 cursor-pointer">
							<input
								type="checkbox"
								bind:checked={relayRequireCuration}
								class="w-4 h-4 rounded border-surface-600 bg-surface-700 text-primary-500 focus:ring-primary-500"
							/>
							<span class="flex items-center gap-1 text-white">
								<Shield class="w-4 h-4 text-surface-400" />
								Require Curation
							</span>
						</label>
					</div>
					<p class="text-xs text-surface-500 -mt-2">Only accept curated/verified content</p>

					<!-- Enable Discovery -->
					<div class="flex items-center gap-3">
						<label class="flex items-center gap-2 cursor-pointer">
							<input
								type="checkbox"
								bind:checked={relayEnableDiscovery}
								class="w-4 h-4 rounded border-surface-600 bg-surface-700 text-primary-500 focus:ring-primary-500"
							/>
							<span class="text-white">Enable Relay Discovery</span>
						</label>
					</div>
					<p class="text-xs text-surface-500 -mt-2">Announce this relay on Nostr for discovery</p>

					<!-- Sync With Relays -->
					<div>
						<span class="label">Sync With Relays</span>
						<div class="flex flex-wrap gap-2 min-h-[2.5rem] p-2 bg-surface-700 rounded-lg mb-2">
							{#if relaySyncWith.length === 0}
								<span class="text-surface-500 text-sm">No sync relays configured</span>
							{:else}
								{#each relaySyncWith as url}
									<span class="inline-flex items-center gap-1 px-2 py-1 bg-primary-500/20 text-primary-400 rounded-md text-sm font-mono">
										{url}
										<button
											onclick={() => removeSyncRelay(url)}
											class="hover:text-red-400 transition-colors"
											aria-label="Remove relay"
										>
											<X class="w-3 h-3" />
										</button>
									</span>
								{/each}
							{/if}
						</div>
						<div class="flex gap-2">
							<input
								type="text"
								bind:value={newSyncRelay}
								onkeydown={handleSyncRelayKeydown}
								placeholder="wss://relay.example.com"
								class="input font-mono flex-1"
							/>
							<button
								class="btn-secondary"
								onclick={() => addSyncRelay(newSyncRelay)}
								disabled={!newSyncRelay.trim()}
							>
								<Plus class="w-4 h-4" />
								Add
							</button>
						</div>
						<p class="text-xs text-surface-500 mt-1">Relay URLs to sync events with</p>
					</div>
				</div>
			{/if}

			<button class="btn-primary" onclick={updateRelaySettings}>
				Save Relay Settings
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
