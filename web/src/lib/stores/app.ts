import { writable, derived } from 'svelte/store';
import type { StatsResponse, SetupStatus, IndexerStatus, TorrentSummary } from '$lib/api/client';

// Setup status
export const setupStatus = writable<SetupStatus | null>(null);

// Stats
export const stats = writable<StatsResponse | null>(null);

// Indexer status
export const indexerStatus = writable<IndexerStatus | null>(null);

// Search results
export const searchResults = writable<TorrentSummary[]>([]);
export const searchQuery = writable('');
export const searchLoading = writable(false);
export const searchTotal = writable(0);

// UI State
export const sidebarCollapsed = writable(false);
export const currentView = writable<'grid' | 'list'>('list');

// Toast notifications
export interface Toast {
	id: number;
	type: 'success' | 'error' | 'info';
	message: string;
}

let toastId = 0;
export const toasts = writable<Toast[]>([]);

export function addToast(type: Toast['type'], message: string, duration = 5000) {
	const id = ++toastId;
	toasts.update((t) => [...t, { id, type, message }]);

	if (duration > 0) {
		setTimeout(() => {
			toasts.update((t) => t.filter((toast) => toast.id !== id));
		}, duration);
	}

	return id;
}

export function removeToast(id: number) {
	toasts.update((t) => t.filter((toast) => toast.id !== id));
}

// Derived stores
export const isFirstRun = derived(setupStatus, ($status) => $status && !$status.completed);

export const hasIdentity = derived(setupStatus, ($status) => $status?.has_identity ?? false);

export const isIndexerRunning = derived(indexerStatus, ($status) => $status?.running ?? false);

// Format helpers
export function formatBytes(bytes: number): string {
	if (bytes === 0) return '0 B';
	const k = 1024;
	const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
	const i = Math.floor(Math.log(bytes) / Math.log(k));
	return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

export function formatNumber(num: number): string {
	return new Intl.NumberFormat().format(num);
}

export function formatDate(date: string): string {
	return new Date(date).toLocaleDateString('en-US', {
		year: 'numeric',
		month: 'short',
		day: 'numeric'
	});
}

export function formatDateTime(date: string): string {
	return new Date(date).toLocaleString('en-US', {
		year: 'numeric',
		month: 'short',
		day: 'numeric',
		hour: '2-digit',
		minute: '2-digit'
	});
}

// Full category tree mapping (Torznab standard)
const categoryNames: Record<number, string> = {
	// Console
	1000: 'Console',
	1010: 'Console/NDS',
	1020: 'Console/PSP',
	1030: 'Console/Wii',
	1040: 'Console/Xbox',
	1050: 'Console/Xbox360',
	1060: 'Console/Wiiware',
	1070: 'Console/Xbox360DLC',
	1080: 'Console/PS3',
	1090: 'Console/Other',
	1110: 'Console/3DS',
	1120: 'Console/PSVita',
	1130: 'Console/WiiU',
	1140: 'Console/XboxOne',
	1180: 'Console/PS4',

	// Movies
	2000: 'Movies',
	2010: 'Movies/Foreign',
	2020: 'Movies/Other',
	2030: 'Movies/SD',
	2040: 'Movies/HD',
	2045: 'Movies/UHD',
	2050: 'Movies/3D',
	2060: 'Movies/BluRay',
	2070: 'Movies/DVD',
	2080: 'Movies/WEB-DL',

	// Audio
	3000: 'Audio',
	3010: 'Audio/MP3',
	3020: 'Audio/Video',
	3030: 'Audio/Audiobook',
	3040: 'Audio/Lossless',
	3050: 'Audio/Other',
	3060: 'Audio/Foreign',

	// PC
	4000: 'PC',
	4010: 'PC/0day',
	4020: 'PC/ISO',
	4030: 'PC/Mac',
	4040: 'PC/Mobile-Other',
	4050: 'PC/Games',
	4060: 'PC/Mobile-iOS',
	4070: 'PC/Mobile-Android',

	// TV
	5000: 'TV',
	5010: 'TV/WEB-DL',
	5020: 'TV/Foreign',
	5030: 'TV/SD',
	5040: 'TV/HD',
	5045: 'TV/UHD',
	5050: 'TV/Other',
	5060: 'TV/Sport',
	5070: 'TV/Anime',
	5080: 'TV/Documentary',

	// XXX
	6000: 'XXX',
	6010: 'XXX/DVD',
	6020: 'XXX/WMV',
	6030: 'XXX/XviD',
	6040: 'XXX/x264',
	6045: 'XXX/UHD',
	6050: 'XXX/Pack',
	6060: 'XXX/Imageset',
	6070: 'XXX/Other',

	// Books
	7000: 'Books',
	7010: 'Books/Mags',
	7020: 'Books/EBook',
	7030: 'Books/Comics',
	7040: 'Books/Technical',
	7050: 'Books/Other',
	7060: 'Books/Foreign',

	// Other
	8000: 'Other',
	8010: 'Other/Misc',
	8020: 'Other/Hashed'
};

export function getCategoryName(code: number): string {
	// First try exact match for subcategory
	if (categoryNames[code]) {
		return categoryNames[code];
	}
	// Fall back to base category
	const base = Math.floor(code / 1000) * 1000;
	return categoryNames[base] || 'Other';
}

// Get base category name only (for grouping)
export function getBaseCategoryName(code: number): string {
	const base = Math.floor(code / 1000) * 1000;
	return categoryNames[base] || 'Other';
}

export function getCategoryColor(code: number): string {
	const colors: Record<number, string> = {
		1000: 'bg-purple-500',
		2000: 'bg-blue-500',
		3000: 'bg-green-500',
		4000: 'bg-orange-500',
		5000: 'bg-pink-500',
		6000: 'bg-red-500',
		7000: 'bg-yellow-500',
		8000: 'bg-gray-500'
	};

	const base = Math.floor(code / 1000) * 1000;
	return colors[base] || 'bg-gray-500';
}

export function truncateNpub(npub: string): string {
	if (!npub || npub.length < 20) return npub;
	return `${npub.slice(0, 10)}...${npub.slice(-6)}`;
}
