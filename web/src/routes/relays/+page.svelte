<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import {
		Radio,
		Plus,
		Trash2,
		Power,
		PowerOff,
		CheckCircle,
		XCircle,
		AlertCircle,
		X,
		Globe
	} from 'lucide-svelte';
	import { api } from '$lib/api/client';
	import type { Relay } from '$lib/api/client';
	import { addToast, formatDateTime } from '$lib/stores/app';

	let relays: Relay[] = [];
	let showAddRelay = false;
	let newRelayUrl = '';
	let newRelayName = '';
	let newRelayPreset = 'public';
	let refreshInterval: ReturnType<typeof setInterval>;

	const presets = [
		{ value: 'public', label: 'Public', description: 'General purpose relay' },
		{ value: 'private', label: 'Private', description: 'Private/invite-only relay' },
		{ value: 'censorship-resistant', label: 'Censorship Resistant', description: 'No content moderation' }
	];

	onMount(async () => {
		await loadRelays();
		// Auto-refresh relay status every 5 seconds
		refreshInterval = setInterval(loadRelays, 5000);
	});

	onDestroy(() => {
		if (refreshInterval) {
			clearInterval(refreshInterval);
		}
	});

	async function loadRelays() {
		try {
			relays = await api.getRelays();
		} catch (error) {
			console.error('Failed to load relays:', error);
		}
	}

	async function addRelay() {
		if (!newRelayUrl) return;

		try {
			await api.addRelay(newRelayUrl, newRelayName || undefined, newRelayPreset, true);
			addToast('success', 'Relay added');
			showAddRelay = false;
			newRelayUrl = '';
			newRelayName = '';
			newRelayPreset = 'public';
			await loadRelays();
		} catch (error) {
			addToast('error', 'Failed to add relay');
		}
	}

	async function deleteRelay(id: number) {
		if (!confirm('Remove this relay?')) return;

		try {
			await api.deleteRelay(id);
			addToast('success', 'Relay removed');
			await loadRelays();
		} catch (error) {
			addToast('error', 'Failed to remove relay');
		}
	}

	async function toggleRelay(relay: Relay) {
		try {
			await api.updateRelay(relay.id, { enabled: !relay.enabled });
			await loadRelays();
		} catch (error) {
			addToast('error', 'Failed to update relay');
		}
	}

	async function connectRelay(id: number) {
		try {
			await api.connectRelay(id);
			addToast('info', 'Connecting to relay...');
			setTimeout(loadRelays, 2000);
		} catch (error) {
			addToast('error', 'Failed to connect to relay');
		}
	}

	async function disconnectRelay(id: number) {
		try {
			await api.disconnectRelay(id);
			addToast('success', 'Disconnected from relay');
			await loadRelays();
		} catch (error) {
			addToast('error', 'Failed to disconnect from relay');
		}
	}

	function getStatusIcon(status: string) {
		switch (status) {
			case 'connected':
				return { icon: CheckCircle, class: 'text-green-400' };
			case 'connecting':
				return { icon: AlertCircle, class: 'text-yellow-400 animate-pulse' };
			case 'error':
				return { icon: XCircle, class: 'text-red-400' };
			default:
				return { icon: XCircle, class: 'text-surface-500' };
		}
	}
</script>

<div class="page-header">
	<h1 class="text-2xl font-bold text-white">Nostr Relays</h1>
	<p class="text-surface-400 mt-1">Manage your Nostr relay connections</p>
</div>

<div class="page-content">
	<!-- Stats -->
	<div class="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-6">
		<div class="stat-card">
			<div class="flex items-center gap-3">
				<div class="p-2 bg-green-900/50 rounded-lg">
					<Radio class="w-5 h-5 text-green-400" />
				</div>
				<div>
					<p class="stat-value">{relays.filter(r => r.status === 'connected').length}</p>
					<p class="stat-label">Connected</p>
				</div>
			</div>
		</div>
		<div class="stat-card">
			<div class="flex items-center gap-3">
				<div class="p-2 bg-yellow-900/50 rounded-lg">
					<AlertCircle class="w-5 h-5 text-yellow-400" />
				</div>
				<div>
					<p class="stat-value">{relays.filter(r => r.status === 'error').length}</p>
					<p class="stat-label">Errors</p>
				</div>
			</div>
		</div>
		<div class="stat-card">
			<div class="flex items-center gap-3">
				<div class="p-2 bg-surface-800 rounded-lg">
					<Globe class="w-5 h-5 text-surface-400" />
				</div>
				<div>
					<p class="stat-value">{relays.length}</p>
					<p class="stat-label">Total Relays</p>
				</div>
			</div>
		</div>
	</div>

	<!-- Relay list -->
	<div class="card">
		<div class="flex items-center justify-between mb-4">
			<h2 class="text-lg font-semibold text-white">Configured Relays</h2>
			<button class="btn-primary" onclick={() => (showAddRelay = true)}>
				<Plus class="w-4 h-4" />
				Add Relay
			</button>
		</div>

		{#if relays.length > 0}
			<div class="space-y-3">
				{#each relays as relay}
					{@const status = getStatusIcon(relay.status)}
					<div class="p-4 bg-surface-800 rounded-lg">
						<div class="flex items-start justify-between gap-4">
							<div class="flex items-start gap-3">
								<div class="mt-1">
									<svelte:component this={status.icon} class="w-5 h-5 {status.class}" />
								</div>
								<div>
									<div class="flex items-center gap-2">
										<span class="font-medium text-surface-100">
											{relay.name || relay.url}
										</span>
										{#if relay.preset}
											<span class="badge badge-primary">{relay.preset}</span>
										{/if}
									</div>
									<code class="text-xs text-surface-500 font-mono mt-1 block">
										{relay.url}
									</code>
									{#if relay.last_connected_at}
										<p class="text-xs text-surface-500 mt-1">
											Last connected: {formatDateTime(relay.last_connected_at)}
										</p>
									{/if}
								</div>
							</div>

							<div class="flex items-center gap-2">
								{#if relay.status === 'connected'}
									<button
										class="btn-icon btn-ghost text-yellow-400"
										title="Disconnect"
										onclick={() => disconnectRelay(relay.id)}
									>
										<PowerOff class="w-4 h-4" />
									</button>
								{:else}
									<button
										class="btn-icon btn-ghost text-green-400"
										title="Connect"
										onclick={() => connectRelay(relay.id)}
									>
										<Power class="w-4 h-4" />
									</button>
								{/if}

								<button
									class="btn-icon btn-ghost {relay.enabled ? 'text-surface-400' : 'text-red-400'}"
									title={relay.enabled ? 'Enabled' : 'Disabled'}
									onclick={() => toggleRelay(relay)}
								>
									{#if relay.enabled}
										<CheckCircle class="w-4 h-4" />
									{:else}
										<XCircle class="w-4 h-4" />
									{/if}
								</button>

								<button
									class="btn-icon btn-ghost text-red-400"
									title="Delete"
									onclick={() => deleteRelay(relay.id)}
								>
									<Trash2 class="w-4 h-4" />
								</button>
							</div>
						</div>
					</div>
				{/each}
			</div>
		{:else}
			<p class="text-surface-500 text-center py-8">No relays configured</p>
		{/if}
	</div>
</div>

<!-- Add Relay Modal -->
{#if showAddRelay}
	<div class="modal-backdrop" onclick={() => (showAddRelay = false)}></div>
	<div class="modal">
		<div class="modal-header">
			<h2 class="text-lg font-semibold text-white">Add Relay</h2>
			<button class="btn-icon btn-ghost" onclick={() => (showAddRelay = false)}>
				<X class="w-5 h-5" />
			</button>
		</div>
		<div class="modal-body space-y-4">
			<div>
				<label class="label" for="relay-url">Relay URL</label>
				<input
					id="relay-url"
					type="text"
					bind:value={newRelayUrl}
					placeholder="wss://relay.example.com"
					class="input font-mono"
				/>
			</div>
			<div>
				<label class="label" for="relay-name">Name (optional)</label>
				<input
					id="relay-name"
					type="text"
					bind:value={newRelayName}
					placeholder="My Relay"
					class="input"
				/>
			</div>
			<div>
				<label class="label">Preset</label>
				<div class="space-y-2">
					{#each presets as preset}
						<label class="flex items-start gap-3 p-3 rounded-lg border cursor-pointer transition-colors {newRelayPreset === preset.value ? 'border-primary-500 bg-primary-900/20' : 'border-surface-700 hover:border-surface-600'}">
							<input
								type="radio"
								name="preset"
								value={preset.value}
								bind:group={newRelayPreset}
								class="mt-1"
							/>
							<div>
								<p class="font-medium text-surface-100">{preset.label}</p>
								<p class="text-xs text-surface-400">{preset.description}</p>
							</div>
						</label>
					{/each}
				</div>
			</div>
		</div>
		<div class="modal-footer">
			<button class="btn-secondary" onclick={() => (showAddRelay = false)}>Cancel</button>
			<button class="btn-primary" onclick={addRelay} disabled={!newRelayUrl}>
				Add Relay
			</button>
		</div>
	</div>
{/if}
