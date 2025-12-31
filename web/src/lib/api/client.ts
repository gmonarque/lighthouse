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

	async discoverUserRelays(npub: string) {
		return this.request<DiscoverRelaysResponse>(`/trust/whitelist/${encodeURIComponent(npub)}/discover-relays`, {
			method: 'POST'
		});
	}

	async discoverAllTrustedRelays() {
		return this.request<DiscoverAllRelaysResponse>('/trust/whitelist/discover-all-relays', {
			method: 'POST'
		});
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

	// Curators (Federated Trust)
	async getCurators() {
		return this.request<CuratorsResponse>('/trust/curators');
	}

	async addCurator(pubkey: string, alias?: string, weight?: number, rulesets?: string[]) {
		return this.request('/trust/curators', {
			method: 'POST',
			body: JSON.stringify({ pubkey, alias, weight, rulesets })
		});
	}

	async updateCurator(pubkey: string, alias?: string, weight?: number) {
		return this.request(`/trust/curators/${encodeURIComponent(pubkey)}`, {
			method: 'PUT',
			body: JSON.stringify({ alias, weight })
		});
	}

	async revokeCurator(pubkey: string, reason?: string) {
		return this.request(`/trust/curators/${encodeURIComponent(pubkey)}`, {
			method: 'DELETE',
			body: JSON.stringify({ reason })
		});
	}

	async getTrustPolicy() {
		return this.request<TrustPolicyResponse>('/trust/policy');
	}

	async getAggregationPolicy() {
		return this.request<AggregationPolicy>('/trust/aggregation');
	}

	async updateAggregationPolicy(mode: string, quorumRequired?: number, weightThreshold?: number) {
		return this.request('/trust/aggregation', {
			method: 'PUT',
			body: JSON.stringify({ mode, quorum_required: quorumRequired, weight_threshold: weightThreshold })
		});
	}

	// Rulesets
	async getRulesets() {
		return this.request<Ruleset[]>('/rulesets');
	}

	async getActiveRulesets() {
		return this.request<ActiveRulesetsResponse>('/rulesets/active');
	}

	async getRuleset(id: string) {
		return this.request<RulesetDetail>(`/rulesets/${encodeURIComponent(id)}`);
	}

	async importRuleset(content: string, source?: string) {
		return this.request<ImportRulesetResponse>('/rulesets', {
			method: 'POST',
			body: JSON.stringify({ content, source })
		});
	}

	async activateRuleset(id: string) {
		return this.request(`/rulesets/${encodeURIComponent(id)}/activate`, { method: 'POST' });
	}

	async deactivateRuleset(id: string) {
		return this.request(`/rulesets/${encodeURIComponent(id)}/deactivate`, { method: 'POST' });
	}

	async deleteRuleset(id: string) {
		return this.request(`/rulesets/${encodeURIComponent(id)}`, { method: 'DELETE' });
	}

	// Decisions
	async getDecisions(params?: DecisionParams) {
		return this.request<DecisionsResponse>('/decisions', { params: params as Record<string, string | number> });
	}

	async getDecisionsByInfohash(infohash: string) {
		return this.request<DecisionsByInfohashResponse>(`/decisions/infohash/${encodeURIComponent(infohash)}`);
	}

	async getDecisionStats() {
		return this.request<DecisionStats>('/decisions/stats');
	}

	async getReasonCodes() {
		return this.request<ReasonCode[]>('/decisions/reason-codes');
	}

	// Reports & Appeals
	async getReports(params?: ReportParams) {
		return this.request<ReportsResponse>('/reports', { params: params as Record<string, string | number> });
	}

	async getPendingReports() {
		return this.request<ReportsResponse>('/reports/pending');
	}

	async getReport(id: string) {
		return this.request<Report>(`/reports/${encodeURIComponent(id)}`);
	}

	async submitReport(data: SubmitReportRequest) {
		return this.request<SubmitReportResponse>('/reports', {
			method: 'POST',
			body: JSON.stringify(data)
		});
	}

	async updateReport(id: string, status: string, resolution?: string, resolvedBy?: string) {
		return this.request(`/reports/${encodeURIComponent(id)}`, {
			method: 'PUT',
			body: JSON.stringify({ status, resolution, resolved_by: resolvedBy })
		});
	}

	async acknowledgeReport(id: string) {
		return this.request(`/reports/${encodeURIComponent(id)}/acknowledge`, { method: 'POST' });
	}

	// Comments
	async getCommentsByInfohash(infohash: string, limit?: number, offset?: number) {
		return this.request<CommentsResponse>(`/torrents/${encodeURIComponent(infohash)}/comments`, {
			params: { limit, offset }
		});
	}

	async addComment(infohash: string, content: string, rating?: number, parentId?: string, authorPubkey?: string) {
		return this.request<AddCommentResponse>(`/torrents/${encodeURIComponent(infohash)}/comments`, {
			method: 'POST',
			body: JSON.stringify({
				content,
				rating,
				parent_id: parentId,
				author_pubkey: authorPubkey
			})
		});
	}

	async getCommentStats(infohash: string) {
		return this.request<CommentStats>(`/torrents/${encodeURIComponent(infohash)}/comments/stats`);
	}

	async getRecentComments(limit?: number) {
		return this.request<CommentsResponse>('/comments/recent', { params: { limit } });
	}

	async getComment(eventId: string) {
		return this.request<Comment>(`/comments/${encodeURIComponent(eventId)}`);
	}

	async getCommentThread(eventId: string) {
		return this.request<CommentThread>(`/comments/${encodeURIComponent(eventId)}/thread`);
	}

	async deleteComment(eventId: string) {
		return this.request(`/comments/${encodeURIComponent(eventId)}`, { method: 'DELETE' });
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

	async updateSettings(settings: Record<string, unknown>) {
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

	// API Keys
	async getAPIKeys() {
		return this.request<APIKeysResponse>('/apikeys');
	}

	async createAPIKey(data: CreateAPIKeyRequest) {
		return this.request<CreateAPIKeyResponse>('/apikeys', {
			method: 'POST',
			body: JSON.stringify(data)
		});
	}

	async getAPIKey(id: string) {
		return this.request<APIKeyDetail>(`/apikeys/${encodeURIComponent(id)}`);
	}

	async updateAPIKey(id: string, data: UpdateAPIKeyRequest) {
		return this.request(`/apikeys/${encodeURIComponent(id)}`, {
			method: 'PUT',
			body: JSON.stringify(data)
		});
	}

	async deleteAPIKey(id: string) {
		return this.request(`/apikeys/${encodeURIComponent(id)}`, { method: 'DELETE' });
	}

	async enableAPIKey(id: string) {
		return this.request(`/apikeys/${encodeURIComponent(id)}/enable`, { method: 'POST' });
	}

	async disableAPIKey(id: string) {
		return this.request(`/apikeys/${encodeURIComponent(id)}/disable`, { method: 'POST' });
	}

	async getAvailablePermissions() {
		return this.request<PermissionInfo[]>('/apikeys/permissions');
	}

	// Explorer Stats
	async getExplorerStats() {
		return this.request<ExplorerStats>('/explorer/stats');
	}

	// SLA
	async getSLAStatus() {
		return this.request<SLAStatus>('/sla/status');
	}

	async getSLAHistory() {
		return this.request<SLAHistory>('/sla/history');
	}

	// Indexer
	async startIndexer() {
		return this.request('/indexer/start', { method: 'POST' });
	}

	async stopIndexer() {
		return this.request('/indexer/stop', { method: 'POST' });
	}

	async resyncIndexer(days: number = 30) {
		return this.request('/indexer/resync', { method: 'POST', params: { days } });
	}

	async getIndexerStatus() {
		return this.request<IndexerStatus>('/indexer/status');
	}

	// Publish
	async parseTorrentFile(file: File): Promise<TorrentFileInfo> {
		await this.ensureInit();

		const formData = new FormData();
		formData.append('file', file);

		const headers: Record<string, string> = {};
		if (this.apiKey) {
			headers['X-API-Key'] = this.apiKey;
		}

		const response = await fetch(`${API_BASE}/publish/parse-torrent`, {
			method: 'POST',
			headers,
			body: formData
		});

		if (!response.ok) {
			const error = await response.json().catch(() => ({ error: 'Unknown error' }));
			throw new Error(error.error || `HTTP ${response.status}`);
		}

		return response.json();
	}

	async publishTorrent(data: PublishTorrentRequest): Promise<PublishTorrentResponse> {
		return this.request<PublishTorrentResponse>('/publish', {
			method: 'POST',
			body: JSON.stringify(data)
		});
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

export interface DiscoverRelaysResponse {
	npub: string;
	relays_added: number;
	status: string;
}

export interface DiscoverAllRelaysResponse {
	users_processed: number;
	relays_added: number;
	status: string;
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
	relay: {
		enabled: boolean;
		listen: string;
		mode: 'public' | 'community';
		require_curation: boolean;
		sync_with: string[];
		enable_discovery: boolean;
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

export interface TorrentFile {
	name: string;
	size: number;
}

export interface TorrentFileInfo {
	info_hash: string;
	name: string;
	size: number;
	files: TorrentFile[];
	trackers: string[];
	comment: string;
}

export interface PublishTorrentRequest {
	info_hash: string;
	name: string;
	size: number;
	category?: number;
	files?: TorrentFile[];
	trackers?: string[];
	tags?: string[];
	description?: string;
	imdb_id?: string;
	tmdb_id?: string;
	relay_ids?: number[];
}

export interface PublishResult {
	relay_id: number;
	relay_url: string;
	success: boolean;
	error?: string;
}

export interface PublishTorrentResponse {
	event_id: string;
	results: PublishResult[];
}

// Curator types
export interface Curator {
	pubkey: string;
	alias?: string;
	weight: number;
	status: string;
	approved_at?: string;
	revoked_at?: string;
	revoke_reason?: string;
	trusted_rulesets?: string[];
}

export interface CuratorsResponse {
	curators: Curator[];
	total: number;
}

export interface TrustPolicyResponse {
	policy: TrustPolicy | null;
}

export interface TrustPolicy {
	policy_id: string;
	version: number;
	allowlist: CuratorEntry[];
	revoked: RevokedKey[];
	created_at: string;
}

export interface CuratorEntry {
	pubkey: string;
	alias?: string;
	weight: number;
	added_at: string;
}

export interface RevokedKey {
	pubkey: string;
	reason?: string;
	revoked_at: string;
}

export interface AggregationPolicy {
	mode: string;
	quorum_required: number;
	weight_threshold: number;
}

// Ruleset types
export interface Ruleset {
	id: string;
	type: string;
	version: string;
	hash: string;
	description?: string;
	rule_count?: number;
	is_active: boolean;
	source?: string;
	created_at?: string;
}

export interface RulesetDetail extends Ruleset {
	rules?: Rule[];
}

export interface Rule {
	id: string;
	type: string;
	action: string;
	reason_code: string;
	conditions?: RuleCondition[];
}

export interface RuleCondition {
	field: string;
	operator: string;
	value: string | string[] | number;
}

export interface ActiveRulesetsResponse {
	censoring?: Ruleset;
	semantic?: Ruleset;
}

export interface ImportRulesetResponse {
	id: string;
	version: string;
	hash: string;
	message: string;
}

// Decision types
export interface Decision {
	decision_id: string;
	decision: string;
	reason_codes: string[];
	target_event_id: string;
	target_infohash: string;
	curator_pubkey: string;
	ruleset_type?: string;
	ruleset_version?: string;
	ruleset_hash?: string;
	created_at: string;
	signature?: string;
}

export interface DecisionsResponse {
	decisions: Decision[];
	total: number;
}

export interface DecisionsByInfohashResponse {
	decisions: Decision[];
	summary: DecisionSummary;
}

export interface DecisionSummary {
	total_decisions: number;
	accept_count: number;
	reject_count: number;
	has_legal_reject: boolean;
	final_decision: string;
}

export interface DecisionStats {
	total_decisions: number;
	accept_count: number;
	reject_count: number;
	unique_curators: number;
}

export interface DecisionParams {
	infohash?: string;
	curator?: string;
	decision?: string;
	limit?: number;
	offset?: number;
}

export interface ReasonCode {
	code: string;
	category: string;
	deterministic: boolean;
	description: string;
}

// Report types
export interface Report {
	id: string;
	report_id: string;
	kind: string;
	target_event_id?: string;
	target_infohash?: string;
	category: string;
	evidence?: string;
	scope?: string;
	jurisdiction?: string;
	reporter_pubkey: string;
	status: string;
	resolution?: string;
	created_at: string;
	acknowledged_at?: string;
	resolved_at?: string;
}

export interface ReportsResponse {
	reports: Report[];
	total: number;
	stats?: ReportStats;
}

export interface ReportStats {
	pending: number;
	acknowledged: number;
	resolved: number;
	rejected: number;
}

export interface ReportParams {
	status?: string;
	category?: string;
	infohash?: string;
	limit?: number;
	offset?: number;
}

export interface SubmitReportRequest {
	kind?: string;
	target_event_id?: string;
	target_infohash?: string;
	category: string;
	evidence?: string;
	scope?: string;
	jurisdiction?: string;
	reporter_pubkey?: string;
}

export interface SubmitReportResponse {
	report_id: string;
	message: string;
}

// Comment types
export interface Comment {
	id: string;
	event_id: string;
	infohash: string;
	torrent_event_id?: string;
	author_pubkey: string;
	content: string;
	rating?: number;
	parent_id?: string;
	root_id?: string;
	mentions?: string[];
	created_at: string;
}

export interface CommentsResponse {
	comments: Comment[];
	stats?: CommentStats;
	total?: number;
}

export interface CommentStats {
	total_comments: number;
	total_ratings: number;
	average_rating?: number;
	rating_counts?: Record<number, number>;
}

export interface CommentThread {
	root: Comment;
	replies: Comment[];
	depth?: number;
}

export interface AddCommentResponse {
	event_id: string;
	message: string;
}

// API Key types
export interface APIKeyDetail {
	id: string;
	name: string;
	key_prefix: string;
	permissions: string[];
	rate_limit?: number;
	created_by?: string;
	created_at: string;
	last_used_at?: string;
	expires_at?: string;
	enabled: boolean;
	notes?: string;
}

export interface APIKeysResponse {
	keys: APIKeyDetail[];
	total: number;
}

export interface CreateAPIKeyRequest {
	name: string;
	permissions: string[];
	rate_limit?: number;
	expires_in?: number;
	notes?: string;
}

export interface CreateAPIKeyResponse {
	id: string;
	name: string;
	key: string;
	key_prefix: string;
	message: string;
}

export interface UpdateAPIKeyRequest {
	name?: string;
	permissions?: string[];
	rate_limit?: number;
	notes?: string;
}

export interface PermissionInfo {
	id: string;
	name: string;
	description: string;
}

// Explorer Stats types
export interface ExplorerStats {
	events_discovered: number;
	events_last_hour: number;
	events_last_24h: number;
	total_torrents: number;
	connected_relays: number;
	unique_uploaders: number;
	database_size: number;
	queue_length: number;
	events_dropped: number;
	event_types?: { type: string; count: number }[];
	hourly_activity?: { hour: string; count: number }[];
}

// SLA types
export interface SLAStatus {
	reports: {
		total: number;
		pending: number;
		acknowledged_within_sla: number;
		past_sla: number;
		avg_ack_time_hours: number;
		sla_compliance_rate: number;
	};
	resolution: {
		total_resolved: number;
		avg_resolution_time_hours: number;
		by_status: Record<string, number>;
	};
	system: {
		uptime_percent: number;
		last_check: string;
		recent_errors: number;
	};
	thresholds: {
		acknowledgment_hours: number;
		resolution_hours: number;
		uptime_percent: number;
	};
	overall_compliant: boolean;
	status: string;
}

export interface SLAHistory {
	history: {
		date: string;
		total_reports: number;
		within_sla: number;
		compliance_rate: number;
	}[];
	period: string;
}

export const api = new APIClient();
