<script lang="ts">
	import { onMount } from 'svelte';
	import {
		Upload,
		FileText,
		Plus,
		Trash2,
		CheckCircle,
		XCircle,
		Film,
		Tv,
		Music,
		Gamepad2,
		BookOpen,
		Radio
	} from 'lucide-svelte';
	import { api } from '$lib/api/client';
	import type { Relay, TorrentFile, PublishResult } from '$lib/api/client';
	import { addToast, formatBytes, getCategoryName } from '$lib/stores/app';

	let relays: Relay[] = [];
	let selectedRelayIds: number[] = [];
	let isPublishing = false;
	let publishResults: PublishResult[] | null = null;
	let publishedEventId = '';

	// Form state
	let infoHash = '';
	let name = '';
	let size = 0;
	let sizeUnit = 'GB';
	let category = 2000;
	let files: TorrentFile[] = [];
	let trackers: string[] = [];
	let tags: string[] = [];
	let description = '';
	let imdbId = '';
	let tmdbId = '';

	// New file/tracker/tag input
	let newFileName = '';
	let newFileSize = 0;
	let newFileSizeUnit = 'MB';
	let newTracker = '';
	let newTag = '';
	let customCategory = '';

	const commonTags = [
		'movie', 'tv', 'music', 'games', 'software', 'books',
		'4k', 'uhd', '2160p', '1080p', '720p', 'hd',
		'bluray', 'web-dl', 'webrip', 'dvdrip',
		'x264', 'x265', 'hevc', 'hdr', 'dts', 'atmos',
		'remux', 'proper', 'repack'
	];

	const categoryOptions = [
		{ value: 2000, label: 'Movies', icon: Film },
		{ value: 2045, label: 'Movies/UHD (4K)', icon: Film },
		{ value: 2040, label: 'Movies/HD', icon: Film },
		{ value: 5000, label: 'TV', icon: Tv },
		{ value: 5045, label: 'TV/UHD (4K)', icon: Tv },
		{ value: 5040, label: 'TV/HD', icon: Tv },
		{ value: 3000, label: 'Audio', icon: Music },
		{ value: 4050, label: 'Games', icon: Gamepad2 },
		{ value: 7000, label: 'Books', icon: BookOpen }
	];

	onMount(async () => {
		await loadRelays();
	});

	async function loadRelays() {
		try {
			relays = await api.getRelays();
			// Select all connected relays by default
			selectedRelayIds = relays
				.filter(r => r.status === 'connected')
				.map(r => r.id);
		} catch (error) {
			console.error('Failed to load relays:', error);
		}
	}

	async function importTorrentFile() {
		const input = document.createElement('input');
		input.type = 'file';
		input.accept = '.torrent';

		input.onchange = async (e) => {
			const file = (e.target as HTMLInputElement).files?.[0];
			if (!file) return;

			try {
				const info = await api.parseTorrentFile(file);

				infoHash = info.info_hash;
				name = info.name;
				size = info.size;
				sizeUnit = 'B';
				files = info.files || [];
				trackers = info.trackers || [];

				if (info.comment) {
					description = info.comment;
				}

				addToast('success', 'Torrent file imported successfully');
			} catch (error) {
				addToast('error', 'Failed to parse torrent file: ' + (error as Error).message);
			}
		};

		input.click();
	}

	function getSizeInBytes(): number {
		const multipliers: Record<string, number> = {
			'B': 1,
			'KB': 1024,
			'MB': 1024 * 1024,
			'GB': 1024 * 1024 * 1024,
			'TB': 1024 * 1024 * 1024 * 1024
		};
		return size * (multipliers[sizeUnit] || 1);
	}

	function addFile() {
		if (!newFileName) return;
		const multipliers: Record<string, number> = {
			'B': 1,
			'KB': 1024,
			'MB': 1024 * 1024,
			'GB': 1024 * 1024 * 1024
		};
		files = [...files, {
			name: newFileName,
			size: newFileSize * (multipliers[newFileSizeUnit] || 1)
		}];
		newFileName = '';
		newFileSize = 0;
	}

	function removeFile(index: number) {
		files = files.filter((_, i) => i !== index);
	}

	function addTracker() {
		if (!newTracker || trackers.includes(newTracker)) return;
		trackers = [...trackers, newTracker];
		newTracker = '';
	}

	function removeTracker(index: number) {
		trackers = trackers.filter((_, i) => i !== index);
	}

	function toggleTag(tag: string) {
		if (tags.includes(tag)) {
			tags = tags.filter(t => t !== tag);
		} else {
			tags = [...tags, tag];
		}
	}

	function addCustomTag() {
		const tag = newTag.trim().toLowerCase();
		if (!tag || tags.includes(tag)) return;
		tags = [...tags, tag];
		newTag = '';
	}

	function removeTag(tag: string) {
		tags = tags.filter(t => t !== tag);
	}

	function getEffectiveCategory(): number {
		if (customCategory) {
			const parsed = parseInt(customCategory, 10);
			if (!isNaN(parsed) && parsed > 0) {
				return parsed;
			}
		}
		return category;
	}

	function toggleRelay(id: number) {
		if (selectedRelayIds.includes(id)) {
			selectedRelayIds = selectedRelayIds.filter(r => r !== id);
		} else {
			selectedRelayIds = [...selectedRelayIds, id];
		}
	}

	function selectAllRelays() {
		selectedRelayIds = relays.filter(r => r.status === 'connected').map(r => r.id);
	}

	function deselectAllRelays() {
		selectedRelayIds = [];
	}

	async function publish() {
		// Validate
		if (!infoHash) {
			addToast('error', 'Info hash is required');
			return;
		}
		if (!/^[a-fA-F0-9]{40}$/.test(infoHash)) {
			addToast('error', 'Info hash must be 40 hex characters');
			return;
		}
		if (!name) {
			addToast('error', 'Name is required');
			return;
		}
		if (getSizeInBytes() <= 0) {
			addToast('error', 'Size must be positive');
			return;
		}
		if (selectedRelayIds.length === 0) {
			addToast('error', 'Select at least one relay');
			return;
		}

		isPublishing = true;
		publishResults = null;

		try {
			const response = await api.publishTorrent({
				info_hash: infoHash.toLowerCase(),
				name,
				size: getSizeInBytes(),
				category: getEffectiveCategory(),
				files: files.length > 0 ? files : undefined,
				trackers: trackers.length > 0 ? trackers : undefined,
				tags: tags.length > 0 ? tags : undefined,
				description: description || undefined,
				imdb_id: imdbId || undefined,
				tmdb_id: tmdbId || undefined,
				relay_ids: selectedRelayIds
			});

			publishedEventId = response.event_id;
			publishResults = response.results;

			const successCount = response.results.filter(r => r.success).length;
			if (successCount > 0) {
				addToast('success', `Published to ${successCount} relay(s)`);
			} else {
				addToast('error', 'Failed to publish to any relay');
			}
		} catch (error) {
			addToast('error', 'Failed to publish: ' + (error as Error).message);
		} finally {
			isPublishing = false;
		}
	}

	function resetForm() {
		infoHash = '';
		name = '';
		size = 0;
		sizeUnit = 'GB';
		category = 2000;
		customCategory = '';
		files = [];
		trackers = [];
		tags = [];
		newTag = '';
		description = '';
		imdbId = '';
		tmdbId = '';
		publishResults = null;
		publishedEventId = '';
	}
</script>

<div class="page-header">
	<h1 class="text-2xl font-bold text-white">Publish Torrent</h1>
	<p class="text-surface-400 mt-1">Publish a torrent to Nostr relays</p>
</div>

<div class="page-content">
	<!-- Import button -->
	<div class="card mb-6">
		<div class="flex items-center justify-between">
			<div>
				<h2 class="text-lg font-semibold text-white">Import from .torrent file</h2>
				<p class="text-sm text-surface-400 mt-1">Auto-fill form fields from a torrent file</p>
			</div>
			<button class="btn-primary" onclick={importTorrentFile}>
				<Upload class="w-4 h-4" />
				Import .torrent
			</button>
		</div>
	</div>

	<div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
		<!-- Main form -->
		<div class="lg:col-span-2 space-y-6">
			<!-- Basic info -->
			<div class="card">
				<h3 class="text-lg font-semibold text-white mb-4">Basic Information</h3>
				<div class="space-y-4">
					<div>
						<label class="label" for="info-hash">Info Hash *</label>
						<input
							id="info-hash"
							type="text"
							bind:value={infoHash}
							placeholder="40 character hex hash"
							class="input font-mono"
							maxlength="40"
						/>
					</div>

					<div>
						<label class="label" for="name">Name *</label>
						<input
							id="name"
							type="text"
							bind:value={name}
							placeholder="Torrent name"
							class="input"
						/>
					</div>

					<div class="grid grid-cols-2 gap-4">
						<div>
							<label class="label" for="size">Size *</label>
							<div class="flex gap-2">
								<input
									id="size"
									type="number"
									bind:value={size}
									min="0"
									class="input flex-1"
								/>
								<select bind:value={sizeUnit} class="input w-24">
									<option value="B">B</option>
									<option value="KB">KB</option>
									<option value="MB">MB</option>
									<option value="GB">GB</option>
									<option value="TB">TB</option>
								</select>
							</div>
						</div>

						<div>
							<label class="label" for="category">Torznab Category</label>
							<div class="flex gap-2">
								<select id="category" bind:value={category} class="input flex-1" disabled={!!customCategory}>
									{#each categoryOptions as cat}
										<option value={cat.value}>{cat.label} ({cat.value})</option>
									{/each}
								</select>
								<input
									type="number"
									bind:value={customCategory}
									placeholder="Custom code"
									class="input w-40"
									min="1"
								/>
							</div>
							<p class="text-xs text-surface-500 mt-1">Torznab category code for *arr apps compatibility</p>
						</div>
					</div>

					<div>
						<label class="label" for="description">Description</label>
						<textarea
							id="description"
							bind:value={description}
							placeholder="Optional description"
							class="input"
							rows="3"
						></textarea>
					</div>
				</div>
			</div>

			<!-- External IDs -->
			<div class="card">
				<h3 class="text-lg font-semibold text-white mb-4">External IDs</h3>
				<div class="grid grid-cols-2 gap-4">
					<div>
						<label class="label" for="imdb">IMDB ID</label>
						<input
							id="imdb"
							type="text"
							bind:value={imdbId}
							placeholder="tt1234567"
							class="input font-mono"
						/>
					</div>
					<div>
						<label class="label" for="tmdb">TMDB ID</label>
						<input
							id="tmdb"
							type="text"
							bind:value={tmdbId}
							placeholder="movie/12345 or tv/12345"
							class="input font-mono"
						/>
					</div>
				</div>
			</div>

			<!-- Tags -->
			<div class="card">
				<h3 class="text-lg font-semibold text-white mb-4">Tags</h3>

				<!-- Selected tags -->
				{#if tags.length > 0}
					<div class="flex flex-wrap gap-2 mb-4 p-3 bg-surface-800 rounded-lg">
						{#each tags as tag}
							<span class="inline-flex items-center gap-1 px-3 py-1 text-sm bg-primary-600 text-white rounded-full">
								{tag}
								<button
									class="hover:bg-primary-500 rounded-full p-0.5"
									onclick={() => removeTag(tag)}
								>
									<Trash2 class="w-3 h-3" />
								</button>
							</span>
						{/each}
					</div>
				{/if}

				<!-- Add custom tag -->
				<div class="flex gap-2 mb-4">
					<input
						type="text"
						bind:value={newTag}
						placeholder="Add custom tag..."
						class="input flex-1"
						onkeydown={(e) => e.key === 'Enter' && addCustomTag()}
					/>
					<button class="btn-secondary" onclick={addCustomTag}>
						<Plus class="w-4 h-4" />
					</button>
				</div>

				<!-- Common tags -->
				<p class="text-xs text-surface-500 mb-2">Common tags (click to add):</p>
				<div class="flex flex-wrap gap-2">
					{#each commonTags.filter(t => !tags.includes(t)) as tag}
						<button
							class="px-3 py-1.5 text-sm rounded-full transition-colors bg-surface-800 text-surface-300 hover:bg-surface-700"
							onclick={() => toggleTag(tag)}
						>
							{tag}
						</button>
					{/each}
				</div>
			</div>

			<!-- Files -->
			<div class="card">
				<h3 class="text-lg font-semibold text-white mb-4">Files ({files.length})</h3>

				{#if files.length > 0}
					<div class="bg-surface-800 rounded-lg mb-4 max-h-60 overflow-y-auto">
						{#each files as file, i}
							<div class="px-3 py-2 flex justify-between items-center border-b border-surface-700 last:border-b-0">
								<div class="flex items-center gap-2 min-w-0 flex-1">
									<FileText class="w-4 h-4 text-surface-500 shrink-0" />
									<span class="text-sm text-surface-300 truncate">{file.name}</span>
								</div>
								<div class="flex items-center gap-2">
									<span class="text-xs text-surface-500">{formatBytes(file.size)}</span>
									<button
										class="p-1 text-red-400 hover:bg-red-900/30 rounded"
										onclick={() => removeFile(i)}
									>
										<Trash2 class="w-3 h-3" />
									</button>
								</div>
							</div>
						{/each}
					</div>
				{/if}

				<div class="flex gap-2">
					<input
						type="text"
						bind:value={newFileName}
						placeholder="File name"
						class="input flex-1"
					/>
					<input
						type="number"
						bind:value={newFileSize}
						placeholder="Size"
						min="0"
						class="input w-24"
					/>
					<select bind:value={newFileSizeUnit} class="input w-20">
						<option value="KB">KB</option>
						<option value="MB">MB</option>
						<option value="GB">GB</option>
					</select>
					<button class="btn-secondary" onclick={addFile}>
						<Plus class="w-4 h-4" />
					</button>
				</div>
			</div>

			<!-- Trackers -->
			<div class="card">
				<h3 class="text-lg font-semibold text-white mb-4">Trackers ({trackers.length})</h3>

				{#if trackers.length > 0}
					<div class="space-y-2 mb-4">
						{#each trackers as tracker, i}
							<div class="flex items-center gap-2 p-2 bg-surface-800 rounded-lg">
								<code class="text-xs text-surface-300 flex-1 truncate">{tracker}</code>
								<button
									class="p-1 text-red-400 hover:bg-red-900/30 rounded"
									onclick={() => removeTracker(i)}
								>
									<Trash2 class="w-3 h-3" />
								</button>
							</div>
						{/each}
					</div>
				{/if}

				<div class="flex gap-2">
					<input
						type="text"
						bind:value={newTracker}
						placeholder="udp://tracker.example.com:1337/announce"
						class="input flex-1 font-mono text-sm"
					/>
					<button class="btn-secondary" onclick={addTracker}>
						<Plus class="w-4 h-4" />
					</button>
				</div>
			</div>
		</div>

		<!-- Sidebar - Relay selection & publish -->
		<div class="space-y-6">
			<!-- Relay selection -->
			<div class="card">
				<div class="flex items-center justify-between mb-4">
					<h3 class="text-lg font-semibold text-white">Publish to Relays</h3>
					<div class="flex gap-2">
						<button class="text-xs text-primary-400 hover:text-primary-300" onclick={selectAllRelays}>
							All
						</button>
						<span class="text-surface-600">|</span>
						<button class="text-xs text-surface-400 hover:text-surface-300" onclick={deselectAllRelays}>
							None
						</button>
					</div>
				</div>

				<div class="space-y-2 max-h-80 overflow-y-auto">
					{#each relays as relay}
						<label class="flex items-center gap-3 p-3 bg-surface-800 rounded-lg cursor-pointer hover:bg-surface-700 transition-colors">
							<input
								type="checkbox"
								checked={selectedRelayIds.includes(relay.id)}
								onchange={() => toggleRelay(relay.id)}
								disabled={relay.status !== 'connected'}
								class="w-4 h-4"
							/>
							<div class="flex-1 min-w-0">
								<div class="flex items-center gap-2">
									<Radio class="w-4 h-4 {relay.status === 'connected' ? 'text-green-400' : 'text-surface-500'}" />
									<span class="text-sm text-surface-200 truncate">{relay.name || relay.url}</span>
								</div>
								<code class="text-xs text-surface-500 truncate block">{relay.url}</code>
							</div>
							{#if relay.status !== 'connected'}
								<span class="text-xs text-surface-500">offline</span>
							{/if}
						</label>
					{/each}
				</div>

				<div class="mt-4 text-sm text-surface-400">
					{selectedRelayIds.length} relay(s) selected
				</div>
			</div>

			<!-- Publish button -->
			<div class="card">
				<button
					class="btn-primary w-full"
					onclick={publish}
					disabled={isPublishing || !infoHash || !name || size <= 0}
				>
					{#if isPublishing}
						Publishing...
					{:else}
						Publish Torrent
					{/if}
				</button>

				<button
					class="btn-ghost w-full mt-2"
					onclick={resetForm}
				>
					Reset Form
				</button>
			</div>

			<!-- Results -->
			{#if publishResults}
				<div class="card">
					<h3 class="text-lg font-semibold text-white mb-4">Results</h3>

					{#if publishedEventId}
						<div class="mb-4">
							<p class="text-sm text-surface-400 mb-1">Event ID:</p>
							<code class="text-xs text-primary-400 break-all">{publishedEventId}</code>
						</div>
					{/if}

					<div class="space-y-2">
						{#each publishResults as result}
							<div class="flex items-center gap-2 p-2 bg-surface-800 rounded-lg">
								{#if result.success}
									<CheckCircle class="w-4 h-4 text-green-400 shrink-0" />
								{:else}
									<XCircle class="w-4 h-4 text-red-400 shrink-0" />
								{/if}
								<div class="flex-1 min-w-0">
									<code class="text-xs text-surface-300 truncate block">{result.relay_url}</code>
									{#if result.error}
										<span class="text-xs text-red-400">{result.error}</span>
									{/if}
								</div>
							</div>
						{/each}
					</div>
				</div>
			{/if}
		</div>
	</div>
</div>
