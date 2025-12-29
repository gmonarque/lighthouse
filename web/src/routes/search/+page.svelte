<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import {
		Search,
		Grid3X3,
		List,
		Filter,
		Download,
		Shield,
		Ban,
		X,
		Film,
		Tv,
		Music,
		Gamepad2,
		BookOpen,
		ChevronLeft,
		ChevronRight,
		Magnet,
		ExternalLink,
		Copy,
		FileText,
		ChevronDown,
		ChevronUp
	} from 'lucide-svelte';
	import { api } from '$lib/api/client';
	import type { TorrentSummary, TorrentDetail } from '$lib/api/client';

	interface TorrentFile {
		name: string;
		size: number;
	}
	import {
		searchResults,
		searchQuery,
		searchLoading,
		searchTotal,
		currentView,
		formatBytes,
		formatDate,
		getCategoryName,
		getCategoryColor,
		addToast
	} from '$lib/stores/app';

	let query = '';
	let category = '';
	let limit = 50;
	let offset = 0;
	let selectedTorrent: TorrentDetail | null = null;
	let showFilters = false;
	let showAllFiles = false;

	// Parse files from JSON string
	function parseFiles(filesJson: string | undefined): TorrentFile[] {
		if (!filesJson) return [];
		try {
			const parsed = JSON.parse(filesJson);
			return Array.isArray(parsed) ? parsed : [];
		} catch {
			return [];
		}
	}

	// Get file extension icon class
	function isVideoFile(filename: string): boolean {
		const videoExts = ['.mkv', '.mp4', '.avi', '.mov', '.wmv', '.flv', '.webm', '.m4v'];
		return videoExts.some(ext => filename.toLowerCase().endsWith(ext));
	}

	// Full category tree with subcategories for filtering
	const categoryGroups = [
		{ label: 'All Categories', value: '' },
		{
			group: 'Movies',
			options: [
				{ value: '2000', label: 'All Movies' },
				{ value: '2045', label: 'Movies/UHD (4K)' },
				{ value: '2040', label: 'Movies/HD' },
				{ value: '2030', label: 'Movies/SD' },
				{ value: '2080', label: 'Movies/WEB-DL' },
				{ value: '2060', label: 'Movies/BluRay' },
				{ value: '2070', label: 'Movies/DVD' },
				{ value: '2050', label: 'Movies/3D' },
				{ value: '2010', label: 'Movies/Foreign' }
			]
		},
		{
			group: 'TV Shows',
			options: [
				{ value: '5000', label: 'All TV' },
				{ value: '5045', label: 'TV/UHD (4K)' },
				{ value: '5040', label: 'TV/HD' },
				{ value: '5030', label: 'TV/SD' },
				{ value: '5010', label: 'TV/WEB-DL' },
				{ value: '5070', label: 'TV/Anime' },
				{ value: '5080', label: 'TV/Documentary' },
				{ value: '5060', label: 'TV/Sport' },
				{ value: '5020', label: 'TV/Foreign' }
			]
		},
		{
			group: 'Audio',
			options: [
				{ value: '3000', label: 'All Audio' },
				{ value: '3040', label: 'Audio/Lossless' },
				{ value: '3010', label: 'Audio/MP3' },
				{ value: '3030', label: 'Audio/Audiobook' },
				{ value: '3020', label: 'Audio/Video' }
			]
		},
		{
			group: 'Games',
			options: [
				{ value: '1000', label: 'All Console' },
				{ value: '4050', label: 'PC/Games' },
				{ value: '1080', label: 'Console/PS3' },
				{ value: '1180', label: 'Console/PS4' },
				{ value: '1040', label: 'Console/Xbox' },
				{ value: '1030', label: 'Console/Wii' }
			]
		},
		{
			group: 'Software',
			options: [
				{ value: '4000', label: 'All PC/Software' },
				{ value: '4020', label: 'PC/ISO' },
				{ value: '4030', label: 'PC/Mac' },
				{ value: '4060', label: 'PC/Mobile-iOS' },
				{ value: '4070', label: 'PC/Mobile-Android' }
			]
		},
		{
			group: 'Books',
			options: [
				{ value: '7000', label: 'All Books' },
				{ value: '7020', label: 'Books/EBook' },
				{ value: '7030', label: 'Books/Comics' },
				{ value: '7010', label: 'Books/Magazines' },
				{ value: '7040', label: 'Books/Technical' }
			]
		},
		{
			group: 'XXX',
			options: [
				{ value: '6000', label: 'All XXX' },
				{ value: '6045', label: 'XXX/UHD' },
				{ value: '6040', label: 'XXX/x264' }
			]
		},
		{
			group: 'Other',
			options: [
				{ value: '8000', label: 'Other' },
				{ value: '8010', label: 'Other/Misc' }
			]
		}
	];

	const categoryIcons: Record<number, typeof Film> = {
		2000: Film,
		5000: Tv,
		3000: Music,
		1000: Gamepad2,
		7000: BookOpen
	};

	onMount(() => {
		// Check for query params
		const urlQuery = $page.url.searchParams.get('q');
		if (urlQuery) {
			query = urlQuery;
			performSearch();
		} else {
			// Load recent torrents
			performSearch();
		}
	});

	async function performSearch() {
		searchLoading.set(true);
		try {
			const response = await api.search({
				q: query || undefined,
				category: category ? Number(category) : undefined,
				limit,
				offset
			});
			searchResults.set(response.results);
			searchTotal.set(response.total);
			searchQuery.set(query);
		} catch (error) {
			console.error('Search failed:', error);
			addToast('error', 'Search failed');
		} finally {
			searchLoading.set(false);
		}
	}

	function handleSearch(e: Event) {
		e.preventDefault();
		offset = 0;
		performSearch();
	}

	function nextPage() {
		if (offset + limit < $searchTotal) {
			offset += limit;
			performSearch();
		}
	}

	function prevPage() {
		if (offset > 0) {
			offset = Math.max(0, offset - limit);
			performSearch();
		}
	}

	async function openTorrent(torrent: TorrentSummary) {
		try {
			showAllFiles = false; // Reset file list expansion
			selectedTorrent = await api.getTorrent(torrent.id);
		} catch (error) {
			addToast('error', 'Failed to load torrent details');
		}
	}

	function closeTorrent() {
		selectedTorrent = null;
	}

	function copyMagnet() {
		if (selectedTorrent?.magnet_uri) {
			navigator.clipboard.writeText(selectedTorrent.magnet_uri);
			addToast('success', 'Magnet link copied to clipboard');
		}
	}

	function openMagnet() {
		if (selectedTorrent?.magnet_uri) {
			window.open(selectedTorrent.magnet_uri, '_blank');
		}
	}

	function copyMagnetLink(e: Event, magnetUri: string) {
		e.stopPropagation();
		navigator.clipboard.writeText(magnetUri);
		addToast('success', 'Magnet link copied to clipboard');
	}

	function openMagnetLink(e: Event, magnetUri: string) {
		e.stopPropagation();
		window.open(magnetUri, '_blank');
	}

	async function blockUploader(npub: string) {
		if (!confirm(`Block this uploader? All their content will be removed.`)) return;

		try {
			await api.addToBlacklist(npub, 'Blocked from search');
			addToast('success', 'Uploader blocked');
			closeTorrent();
			performSearch();
		} catch (error) {
			addToast('error', 'Failed to block uploader');
		}
	}

	$: totalPages = Math.ceil($searchTotal / limit);
	$: currentPage = Math.floor(offset / limit) + 1;
</script>

<div class="page-header">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold text-white">Search</h1>
			<p class="text-surface-400 mt-1">Browse and search indexed torrents</p>
		</div>
		<div class="flex items-center gap-2">
			<button
				class="btn-icon btn-ghost"
				class:bg-surface-800={$currentView === 'list'}
				onclick={() => currentView.set('list')}
				title="List view"
			>
				<List class="w-5 h-5" />
			</button>
			<button
				class="btn-icon btn-ghost"
				class:bg-surface-800={$currentView === 'grid'}
				onclick={() => currentView.set('grid')}
				title="Grid view"
			>
				<Grid3X3 class="w-5 h-5" />
			</button>
		</div>
	</div>
</div>

<div class="page-content">
	<!-- Search form -->
	<form onsubmit={handleSearch} class="mb-6">
		<div class="flex gap-3">
			<div class="flex-1 relative">
				<Search class="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-surface-500" />
				<input
					type="text"
					bind:value={query}
					placeholder="Search torrents..."
					class="input pl-10"
				/>
			</div>
			<button
				type="button"
				class="btn-secondary"
				onclick={() => (showFilters = !showFilters)}
			>
				<Filter class="w-4 h-4" />
				Filters
			</button>
			<button type="submit" class="btn-primary" disabled={$searchLoading}>
				{$searchLoading ? 'Searching...' : 'Search'}
			</button>
		</div>

		{#if showFilters}
			<div class="mt-3 p-4 bg-surface-900 rounded-lg border border-surface-800">
				<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
					<div>
						<label class="label" for="category">Category</label>
						<select
							id="category"
							bind:value={category}
							class="input"
						>
							{#each categoryGroups as item}
								{#if 'group' in item}
									<optgroup label={item.group}>
										{#each item.options as opt}
											<option value={opt.value}>{opt.label}</option>
										{/each}
									</optgroup>
								{:else}
									<option value={item.value}>{item.label}</option>
								{/if}
							{/each}
						</select>
					</div>
				</div>
			</div>
		{/if}
	</form>

	<!-- Results count -->
	<div class="flex items-center justify-between mb-4">
		<p class="text-sm text-surface-400">
			{#if $searchTotal > 0}
				Showing {offset + 1}-{Math.min(offset + limit, $searchTotal)} of {$searchTotal.toLocaleString()} results
			{:else}
				No results
			{/if}
		</p>
	</div>

	<!-- Results -->
	{#if $searchLoading}
		<div class="flex items-center justify-center py-12">
			<div class="animate-spin w-8 h-8 border-2 border-primary-500 border-t-transparent rounded-full"></div>
		</div>
	{:else if $searchResults.length > 0}
		{#if $currentView === 'grid'}
			<div class="poster-grid">
				{#each $searchResults as torrent}
					<div class="poster-card group">
						<button
							class="w-full h-full text-left"
							onclick={() => openTorrent(torrent)}
						>
							{#if torrent.poster_url}
								<img
									src={torrent.poster_url}
									alt={torrent.title || torrent.name}
									loading="lazy"
								/>
							{:else}
								<div class="w-full h-full flex items-center justify-center bg-surface-800">
									<svelte:component
										this={categoryIcons[Math.floor(torrent.category / 1000) * 1000] || Film}
										class="w-12 h-12 text-surface-600"
									/>
								</div>
							{/if}
							<div class="poster-overlay"></div>
							<div class="poster-info">
								<p class="text-sm font-medium line-clamp-2">{torrent.title || torrent.name}</p>
								<div class="flex items-center gap-2 mt-1 text-xs text-surface-400">
									{#if torrent.year}
										<span>{torrent.year}</span>
									{/if}
									<span>{formatBytes(torrent.size)}</span>
								</div>
							</div>
						</button>
						<!-- Magnet buttons overlay -->
						<div class="absolute top-2 right-2 flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity z-20">
							<button
								class="p-1.5 bg-surface-900/90 hover:bg-primary-600 rounded-md transition-colors"
								onclick={(e) => copyMagnetLink(e, torrent.magnet_uri)}
								title="Copy magnet link"
							>
								<Copy class="w-4 h-4 text-white" />
							</button>
							<button
								class="p-1.5 bg-surface-900/90 hover:bg-green-600 rounded-md transition-colors"
								onclick={(e) => openMagnetLink(e, torrent.magnet_uri)}
								title="Open magnet link"
							>
								<Magnet class="w-4 h-4 text-white" />
							</button>
						</div>
					</div>
				{/each}
			</div>
		{:else}
			<div class="table-container">
				<table class="table">
					<thead>
						<tr>
							<th>Name</th>
							<th>Category</th>
							<th>Size</th>
							<th>Added</th>
							<th>Trust</th>
							<th class="w-24">Actions</th>
						</tr>
					</thead>
					<tbody>
						{#each $searchResults as torrent}
							<tr class="cursor-pointer hover:bg-surface-800/50" onclick={() => openTorrent(torrent)}>
								<td>
									<div class="flex items-center gap-3">
										{#if torrent.poster_url}
											<img
												src={torrent.poster_url}
												alt=""
												class="w-10 h-14 object-cover rounded"
											/>
										{/if}
										<div>
											<p class="font-medium text-surface-100 line-clamp-1">
												{torrent.title || torrent.name}
											</p>
											{#if torrent.year}
												<p class="text-xs text-surface-500">{torrent.year}</p>
											{/if}
										</div>
									</div>
								</td>
								<td>
									<span class="badge badge-primary">{getCategoryName(torrent.category)}</span>
								</td>
								<td>{formatBytes(torrent.size)}</td>
								<td>{formatDate(torrent.first_seen_at)}</td>
								<td>
									<div class="flex items-center gap-1">
										<Shield class="w-4 h-4 text-primary-400" />
										{torrent.trust_score}
									</div>
								</td>
								<td>
									<div class="flex items-center gap-1">
										<button
											class="p-1.5 hover:bg-surface-700 rounded transition-colors"
											onclick={(e) => copyMagnetLink(e, torrent.magnet_uri)}
											title="Copy magnet link"
										>
											<Copy class="w-4 h-4 text-surface-400 hover:text-primary-400" />
										</button>
										<button
											class="p-1.5 hover:bg-surface-700 rounded transition-colors"
											onclick={(e) => openMagnetLink(e, torrent.magnet_uri)}
											title="Open magnet link"
										>
											<Magnet class="w-4 h-4 text-surface-400 hover:text-green-400" />
										</button>
									</div>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}

		<!-- Pagination -->
		{#if totalPages > 1}
			<div class="flex items-center justify-center gap-2 mt-6">
				<button
					class="btn-secondary"
					disabled={offset === 0}
					onclick={prevPage}
				>
					<ChevronLeft class="w-4 h-4" />
					Previous
				</button>
				<span class="text-surface-400 px-4">
					Page {currentPage} of {totalPages}
				</span>
				<button
					class="btn-secondary"
					disabled={offset + limit >= $searchTotal}
					onclick={nextPage}
				>
					Next
					<ChevronRight class="w-4 h-4" />
				</button>
			</div>
		{/if}
	{:else}
		<div class="card text-center py-12">
			<Search class="w-12 h-12 text-surface-600 mx-auto mb-3" />
			<p class="text-surface-400">No torrents found</p>
			<p class="text-sm text-surface-500 mt-1">Try a different search query</p>
		</div>
	{/if}
</div>

<!-- Torrent detail modal -->
{#if selectedTorrent}
	<div class="modal-backdrop" onclick={closeTorrent}></div>
	<div class="modal max-w-2xl">
		<div class="modal-header">
			<h2 class="text-lg font-semibold text-white">
				{selectedTorrent.title || selectedTorrent.name}
			</h2>
			<button class="btn-icon btn-ghost" onclick={closeTorrent}>
				<X class="w-5 h-5" />
			</button>
		</div>

		<div class="modal-body">
			<div class="flex gap-4">
				{#if selectedTorrent.poster_url}
					<img
						src={selectedTorrent.poster_url}
						alt=""
						class="w-32 h-48 object-cover rounded-lg"
					/>
				{/if}
				<div class="flex-1">
					{#if selectedTorrent.overview}
						<p class="text-surface-300 text-sm line-clamp-3 mb-3">
							{selectedTorrent.overview}
						</p>
					{/if}
					<div class="grid grid-cols-2 gap-2 text-sm">
						<div>
							<span class="text-surface-500">Size:</span>
							<span class="text-surface-200 ml-1">{formatBytes(selectedTorrent.size)}</span>
						</div>
						<div>
							<span class="text-surface-500">Category:</span>
							<span class="text-surface-200 ml-1">{getCategoryName(selectedTorrent.category)}</span>
						</div>
						{#if selectedTorrent.year}
							<div>
								<span class="text-surface-500">Year:</span>
								<span class="text-surface-200 ml-1">{selectedTorrent.year}</span>
							</div>
						{/if}
						<div>
							<span class="text-surface-500">Trust Score:</span>
							<span class="text-surface-200 ml-1">{selectedTorrent.trust_score}</span>
						</div>
						<div>
							<span class="text-surface-500">Uploaders:</span>
							<span class="text-surface-200 ml-1">{selectedTorrent.upload_count}</span>
						</div>
					</div>
				</div>
			</div>

			<!-- File list -->
			{#if parseFiles(selectedTorrent.files).length > 0}
				{@const files = parseFiles(selectedTorrent.files)}
				<div class="mt-4 pt-4 border-t border-surface-800">
					<h3 class="text-sm font-medium text-surface-400 mb-2">
						Files ({files.length})
					</h3>
					<div class="bg-surface-800 rounded-lg overflow-hidden">
						{#each (showAllFiles ? files : files.slice(0, 5)) as file}
							<div class="px-3 py-2 flex justify-between items-center hover:bg-surface-700 border-b border-surface-700 last:border-b-0">
								<div class="flex items-center gap-2 min-w-0 flex-1">
									<FileText class="w-4 h-4 text-surface-500 shrink-0" />
									<span class="text-sm text-surface-300 truncate" title={file.name}>
										{file.name}
									</span>
								</div>
								<span class="text-xs text-surface-500 ml-2 shrink-0">
									{formatBytes(file.size)}
								</span>
							</div>
						{/each}
					</div>
					{#if files.length > 5}
						<button
							class="mt-2 text-xs text-primary-400 hover:text-primary-300 flex items-center gap-1"
							onclick={() => showAllFiles = !showAllFiles}
						>
							{#if showAllFiles}
								<ChevronUp class="w-3 h-3" />
								Show less
							{:else}
								<ChevronDown class="w-3 h-3" />
								Show {files.length - 5} more files
							{/if}
						</button>
					{/if}
				</div>
			{/if}

			{#if selectedTorrent.uploaders && selectedTorrent.uploaders.length > 0}
				<div class="mt-4 pt-4 border-t border-surface-800">
					<h3 class="text-sm font-medium text-surface-400 mb-2">
						Uploaders ({selectedTorrent.uploaders.length})
					</h3>
					<div class="space-y-3">
						{#each selectedTorrent.uploaders as uploader}
							<div class="p-3 bg-surface-800 rounded-lg">
								<div class="flex items-start justify-between gap-2 mb-2">
									<code class="text-xs text-surface-300 font-mono break-all flex-1">
										{uploader.npub}
									</code>
									<button
										class="btn-ghost text-red-400 text-xs py-1 px-2 shrink-0"
										onclick={() => blockUploader(uploader.npub)}
									>
										<Ban class="w-3 h-3" />
										Block
									</button>
								</div>
								<div class="flex items-center gap-2">
									<a
										href="https://njump.me/{uploader.event_id}"
										target="_blank"
										rel="noopener noreferrer"
										class="text-xs text-primary-400 hover:text-primary-300 flex items-center gap-1"
									>
										<ExternalLink class="w-3 h-3" />
										View on njump
									</a>
									<span class="text-xs text-surface-500">
										via {uploader.relay_url}
									</span>
								</div>
							</div>
						{/each}
					</div>
				</div>
			{/if}
		</div>

		<div class="modal-footer">
			<button class="btn-secondary" onclick={closeTorrent}>Close</button>
			<button class="btn-secondary" onclick={copyMagnet}>
				<Copy class="w-4 h-4" />
				Copy Magnet
			</button>
			<button class="btn-primary" onclick={openMagnet}>
				<Magnet class="w-4 h-4" />
				Open in Client
			</button>
		</div>
	</div>
{/if}
