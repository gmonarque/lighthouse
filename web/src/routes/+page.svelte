<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import {
		Database,
		HardDrive,
		Users,
		Radio,
		Clock,
		Film,
		Tv,
		Music,
		Gamepad2,
		BookOpen,
		Shield,
		Copy,
		Magnet,
		X,
		Ban,
		ExternalLink,
		FileText,
		ChevronDown,
		ChevronUp,
		FileCheck,
		Crown,
		Flag,
		CheckCircle,
		XCircle
	} from 'lucide-svelte';
	import { api } from '$lib/api/client';
	import {
		stats,
		formatBytes,
		formatNumber,
		formatDate,
		getCategoryName,
		getCategoryColor,
		addToast
	} from '$lib/stores/app';
	import type { TorrentSummary, TorrentDetail, DecisionStats, CuratorsResponse, ReportsResponse } from '$lib/api/client';

	interface TorrentFile {
		name: string;
		size: number;
	}

	let recentTorrents: TorrentSummary[] = [];
	let refreshInterval: ReturnType<typeof setInterval>;
	let selectedTorrent: TorrentDetail | null = null;
	let showAllFiles = false;

	// Curation stats
	let decisionStats: DecisionStats | null = null;
	let curatorCount = 0;
	let pendingReports = 0;

	const categoryIcons: Record<number, typeof Film> = {
		2000: Film,
		5000: Tv,
		3000: Music,
		1000: Gamepad2,
		7000: BookOpen
	};

	function parseFiles(filesJson: string | undefined): TorrentFile[] {
		if (!filesJson) return [];
		try {
			const parsed = JSON.parse(filesJson);
			return Array.isArray(parsed) ? parsed : [];
		} catch {
			return [];
		}
	}

	async function loadData() {
		try {
			const [statsData, decisions, curators, reports] = await Promise.all([
				api.getStats(),
				api.getDecisionStats().catch(() => null),
				api.getCurators().catch(() => ({ curators: [], total: 0 })),
				api.getPendingReports().catch(() => ({ reports: [], total: 0 }))
			]);
			stats.set(statsData);
			recentTorrents = statsData.recent_torrents || [];
			decisionStats = decisions;
			curatorCount = curators.total || curators.curators?.length || 0;
			pendingReports = reports.total || reports.reports?.length || 0;
		} catch (error) {
			console.error('Failed to load dashboard data:', error);
		}
	}

	onMount(() => {
		loadData();
		refreshInterval = setInterval(loadData, 5000);
	});

	onDestroy(() => {
		if (refreshInterval) {
			clearInterval(refreshInterval);
		}
	});

	async function openTorrent(torrent: TorrentSummary) {
		try {
			showAllFiles = false;
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
			await api.addToBlacklist(npub, 'Blocked from dashboard');
			addToast('success', 'Uploader blocked');
			closeTorrent();
			loadData();
		} catch (error) {
			addToast('error', 'Failed to block uploader');
		}
	}

	function getCategoryStats(): { name: string; count: number; color: string }[] {
		if (!$stats?.categories) return [];
		return Object.entries($stats.categories)
			.map(([code, count]) => ({
				name: getCategoryName(Number(code)),
				count,
				color: getCategoryColor(Number(code))
			}))
			.sort((a, b) => b.count - a.count)
			.slice(0, 6);
	}
</script>

<div class="page-header">
	<h1 class="text-2xl font-bold text-white">Dashboard</h1>
	<p class="text-surface-400 mt-1">Overview of your Lighthouse indexer</p>
</div>

<div class="page-content">
	<!-- Stats cards -->
	<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
		<div class="stat-card">
			<div class="flex items-center gap-3">
				<div class="p-2 bg-primary-900/50 rounded-lg">
					<Database class="w-5 h-5 text-primary-400" />
				</div>
				<div>
					<p class="stat-value">{formatNumber($stats?.total_torrents ?? 0)}</p>
					<p class="stat-label">Torrents Indexed</p>
				</div>
			</div>
		</div>

		<div class="stat-card">
			<div class="flex items-center gap-3">
				<div class="p-2 bg-accent-900/50 rounded-lg">
					<HardDrive class="w-5 h-5 text-accent-400" />
				</div>
				<div>
					<p class="stat-value">{formatBytes($stats?.total_size ?? 0)}</p>
					<p class="stat-label">Total Content Size</p>
				</div>
			</div>
		</div>

		<div class="stat-card">
			<div class="flex items-center gap-3">
				<div class="p-2 bg-green-900/50 rounded-lg">
					<Radio class="w-5 h-5 text-green-400" />
				</div>
				<div>
					<p class="stat-value">{formatNumber($stats?.connected_relays ?? 0)}</p>
					<p class="stat-label">Connected Relays</p>
				</div>
			</div>
		</div>

		<div class="stat-card">
			<div class="flex items-center gap-3">
				<div class="p-2 bg-yellow-900/50 rounded-lg">
					<Users class="w-5 h-5 text-yellow-400" />
				</div>
				<div>
					<p class="stat-value">{formatNumber($stats?.unique_uploaders ?? 0)}</p>
					<p class="stat-label">Unique Uploaders</p>
				</div>
			</div>
		</div>
	</div>

	<!-- Curation Stats -->
	{#if decisionStats || curatorCount > 0 || pendingReports > 0}
		<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
			<div class="stat-card">
				<div class="flex items-center gap-3">
					<div class="p-2 bg-blue-900/50 rounded-lg">
						<FileCheck class="w-5 h-5 text-blue-400" />
					</div>
					<div>
						<p class="stat-value">{formatNumber(decisionStats?.total_decisions ?? 0)}</p>
						<p class="stat-label">Curation Decisions</p>
					</div>
				</div>
			</div>

			<div class="stat-card">
				<div class="flex items-center gap-3">
					<div class="p-2 bg-green-900/50 rounded-lg">
						<CheckCircle class="w-5 h-5 text-green-400" />
					</div>
					<div>
						<p class="stat-value">{formatNumber(decisionStats?.accept_count ?? 0)}</p>
						<p class="stat-label">Accepted</p>
					</div>
				</div>
			</div>

			<div class="stat-card">
				<div class="flex items-center gap-3">
					<div class="p-2 bg-purple-900/50 rounded-lg">
						<Crown class="w-5 h-5 text-purple-400" />
					</div>
					<div>
						<p class="stat-value">{formatNumber(curatorCount)}</p>
						<p class="stat-label">Trusted Curators</p>
					</div>
				</div>
			</div>

			<a href="/reports" class="stat-card hover:border-yellow-500/50 transition-colors">
				<div class="flex items-center gap-3">
					<div class="p-2 bg-yellow-900/50 rounded-lg">
						<Flag class="w-5 h-5 text-yellow-400" />
					</div>
					<div>
						<p class="stat-value">{formatNumber(pendingReports)}</p>
						<p class="stat-label">Pending Reports</p>
					</div>
				</div>
			</a>
		</div>
	{/if}

	<!-- Categories breakdown -->
	{#if getCategoryStats().length > 0}
		<div class="card mb-8">
			<h2 class="text-lg font-semibold text-white mb-4">Categories</h2>
			<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
				{#each getCategoryStats() as cat}
					{@const total = $stats?.total_torrents || 1}
					{@const percent = ((cat.count / total) * 100).toFixed(1)}
					<div class="p-3 bg-surface-800 rounded-lg">
						<div class="flex items-center justify-between text-sm mb-2">
							<span class="text-surface-200 font-medium">{cat.name}</span>
							<span class="text-surface-400">{formatNumber(cat.count)} ({percent}%)</span>
						</div>
						<div class="h-2 bg-surface-700 rounded-full overflow-hidden">
							<div
								class="{cat.color} h-full rounded-full transition-all"
								style="width: {percent}%"
							></div>
						</div>
					</div>
				{/each}
			</div>
		</div>
	{/if}

	<!-- Recent torrents -->
	<div>
		<div class="flex items-center justify-between mb-4">
			<h2 class="text-lg font-semibold text-white flex items-center gap-2">
				<Clock class="w-5 h-5 text-primary-400" />
				Recent Additions
			</h2>
			<a href="/search" class="text-sm text-primary-400 hover:text-primary-300">
				View all &rarr;
			</a>
		</div>

		{#if recentTorrents.length > 0}
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
						{#each recentTorrents.slice(0, 15) as torrent}
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
		{:else}
			<div class="card text-center py-12">
				<Database class="w-12 h-12 text-surface-600 mx-auto mb-3" />
				<p class="text-surface-400">No torrents indexed yet</p>
				<p class="text-sm text-surface-500 mt-1">Start the indexer to begin collecting content</p>
			</div>
		{/if}
	</div>
</div>

<!-- Torrent detail modal -->
{#if selectedTorrent}
	<div class="modal-backdrop" onclick={closeTorrent} onkeydown={(e) => e.key === 'Escape' && closeTorrent()} role="button" tabindex="-1"></div>
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
