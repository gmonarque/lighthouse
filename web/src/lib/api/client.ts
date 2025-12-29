const API_BASE = '/api';
const API_KEY_STORAGE_KEY = 'lighthouse_api_key';

interface FetchOptions extends RequestInit {
	params?: Record<string, string | number | undefined>;
}

class APIClient {
	private apiKey: string | null = null;
	private initialized: boolean = false;

	constructor() {
		// Try to load API key from localStorage
		if (typeof window !== 'undefined') {
			this.apiKey = localStorage.getItem(API_KEY_STORAGE_KEY);
		}
	}

	setApiKey(key: string) {
		this.apiKey = key;
		if (typeof window !== 'undefined') {
			localStorage.setItem(API_KEY_STORAGE_KEY, key);
		}
	}

	// Initialize API client by fetching the API key
	async init(): Promise<void> {
		if (this.initialized && this.apiKey) {
			return;
		}

		try {
			const response = await fetch(`${API_BASE}/auth/key`);
			if (response.ok) {
				const data = await response.json();
				if (data.api_key) {
					this.setApiKey(data.api_key);
				}
			}
		} catch (error) {
			console.warn('Failed to fetch API key:', error);
		}

		this.initialized = true;
	}

	// Ensure initialized before making requests
	private async ensureInit(): Promise<void> {
		if (!this.initialized || !this.apiKey) {
			await this.init();
		}
	}

	private async request<T>(endpoint: string, options: FetchOptions = {}): Promise<T> {
		// Ensure API key is loaded
		await this.ensureInit();

		const { params, ...fetchOptions } = options;

		let url = `${API_BASE}${endpoint}`;

		if (params) {
			const searchParams = new URLSearchParams();
			Object.entries(params).forEach(([key, value]) => {
				if (value !== undefined && value !== null) {
					searchParams.set(key, String(value));
				}
			});
			const queryString = searchParams.toString();
			if (queryString) {
				url += `?${queryString}`;
			}
		}

		const headers: Record<string, string> = {
			'Content-Type': 'application/json',
			...(options.headers as Record<string, string>)
		};

		if (this.apiKey) {
			headers['X-API-Key'] = this.apiKey;
		}

		const response = await fetch(url, {
			...fetchOptions,
			headers
		});

		if (!response.ok) {
			const error = await response.json().catch(() => ({ error: 'Unknown error' }));
			throw new Error(error.error || `HTTP ${response.status}`);
		}

		return response.json();
	}

	// Stats
	async getStats() {
		return this.request<StatsResponse>('/stats');
	}

	async getStatsChart(days: number = 7) {
		return this.request<ChartResponse>('/stats/chart', { params: { days } });
	}

	// Search
	async search(params: SearchParams) {
		return this.request<SearchResponse>('/search', { params: params as Record<string, string | number> });
	}

	async getTorrent(id: number) {
		return this.request<TorrentDetail>(`/torrents/${id}`);
	}

	async deleteTorrent(id: number) {
		return this.request(`/torrents/${id}`, { method: 'DELETE' });
	}

	// Trust
	async getWhitelist() {
		return this.request<TrustEntry[]>('/trust/whitelist');
	}

	async addToWhitelist(npub: string, alias?: string, notes?: string) {
		return this.request('/trust/whitelist', {
			method: 'POST',
			body: JSON.stringify({ npub, alias, notes })
		});
	}

	async removeFromWhitelist(npub: string) {
		return this.request(`/trust/whitelist/${encodeURIComponent(npub)}`, { method: 'DELETE' });
	}

	async getBlacklist() {
		return this.request<TrustEntry[]>('/trust/blacklist');
	}

	async addToBlacklist(npub: string, reason?: string) {
		return this.request('/trust/blacklist', {
			method: 'POST',
			body: JSON.stringify({ npub, reason })
		});
	}

	async removeFromBlacklist(npub: string) {
		return this.request(`/trust/blacklist/${encodeURIComponent(npub)}`, { method: 'DELETE' });
	}

	async getTrustSettings() {
		return this.request<TrustSettings>('/trust/settings');
	}

	async updateTrustSettings(depth: number) {
		return this.request('/trust/settings', {
			method: 'PUT',
			body: JSON.stringify({ depth })
		});
	}

	// Relays
	async getRelays() {
		return this.request<Relay[]>('/relays');
	}

	async addRelay(url: string, name?: string, preset?: string, enabled: boolean = true) {
		return this.request('/relays', {
			method: 'POST',
			body: JSON.stringify({ url, name, preset, enabled })
		});
	}

	async updateRelay(id: number, data: Partial<Relay>) {
		return this.request(`/relays/${id}`, {
			method: 'PUT',
			body: JSON.stringify(data)
		});
	}

	async deleteRelay(id: number) {
		return this.request(`/relays/${id}`, { method: 'DELETE' });
	}

	async connectRelay(id: number) {
		return this.request(`/relays/${id}/connect`, { method: 'POST' });
	}

	async disconnectRelay(id: number) {
		return this.request(`/relays/${id}/disconnect`, { method: 'POST' });
	}

	// Settings
	async getSettings() {
		return this.request<AppSettings>('/settings');
	}

	async updateSettings(settings: Partial<AppSettings>) {
		return this.request('/settings', {
			method: 'PUT',
			body: JSON.stringify(settings)
		});
	}

	async generateIdentity() {
		return this.request<IdentityResponse>('/settings/identity/generate', { method: 'POST' });
	}

	async importIdentity(nsec: string) {
		return this.request<IdentityResponse>('/settings/identity/import', {
			method: 'POST',
			body: JSON.stringify({ nsec })
		});
	}

	async exportConfig() {
		return this.request('/settings/export');
	}

	async importConfig(config: unknown) {
		return this.request('/settings/import', {
			method: 'POST',
			body: JSON.stringify(config)
		});
	}

	// Setup
	async getSetupStatus() {
		return this.request<SetupStatus>('/setup/status');
	}

	async completeSetup() {
		return this.request('/setup/complete', { method: 'POST' });
	}

	// Activity
	async getActivity(limit: number = 50, offset: number = 0, type?: string) {
		return this.request<Activity[]>('/activity', { params: { limit, offset, type } });
	}

	// Indexer
	async startIndexer() {
		return this.request('/indexer/start', { method: 'POST' });
	}

	async stopIndexer() {
		return this.request('/indexer/stop', { method: 'POST' });
	}

	async getIndexerStatus() {
		return this.request<IndexerStatus>('/indexer/status');
	}
}

// Types
export interface StatsResponse {
	total_torrents: number;
	total_size: number;
	categories: Record<number, number>;
	connected_relays: number;
	whitelist_count: number;
	blacklist_count: number;
	unique_uploaders: number;
	database_size: number;
	recent_torrents: TorrentSummary[];
}

export interface ChartResponse {
	days: number;
	data: { date: string; count: number }[];
}

export interface SearchParams {
	q?: string;
	category?: number;
	limit?: number;
	offset?: number;
}

export interface SearchResponse {
	results: TorrentSummary[];
	total: number;
	limit: number;
	offset: number;
}

export interface TorrentSummary {
	id: number;
	info_hash: string;
	name: string;
	size: number;
	category: number;
	seeders: number;
	leechers: number;
	magnet_uri: string;
	title: string;
	year: number;
	poster_url: string;
	trust_score: number;
	first_seen_at: string;
}

export interface TorrentDetail extends TorrentSummary {
	magnet_uri: string;
	files: string;
	tmdb_id: number;
	imdb_id: string;
	backdrop_url: string;
	overview: string;
	genres: string;
	rating: number;
	upload_count: number;
	uploaders: Uploader[];
	updated_at: string;
}

export interface Uploader {
	npub: string;
	event_id: string;
	relay_url: string;
	uploaded_at: string;
}

export interface TrustEntry {
	id: number;
	npub: string;
	alias?: string;
	notes?: string;
	reason?: string;
	added_at: string;
}

export interface TrustSettings {
	depth: number;
}

export interface Relay {
	id: number;
	url: string;
	name: string;
	preset: string;
	enabled: boolean;
	status: string;
	last_connected_at: string;
	created_at: string;
}

export interface AppSettings {
	server: {
		host: string;
		port: number;
		api_key: string;
	};
	nostr: {
		identity: {
			npub: string;
			nsec: string;
		};
		relays: Relay[];
	};
	trust: {
		depth: number;
	};
	enrichment: {
		enabled: boolean;
		tmdb_api_key: boolean;
		omdb_api_key: boolean;
	};
	indexer: {
		tag_filter: string[];
		tag_filter_enabled: boolean;
	};
}

export interface IdentityResponse {
	npub: string;
	nsec?: string;
}

export interface SetupStatus {
	completed: boolean;
	has_identity: boolean;
	has_relays: boolean;
	has_tmdb_key: boolean;
	enrichment_enabled: boolean;
}

export interface Activity {
	id: number;
	event_type: string;
	details?: string;
	created_at: string;
}

export interface IndexerStatus {
	running: boolean;
	enabled: boolean;
	total_torrents: number;
	connected_relays: number;
}

export const api = new APIClient();
