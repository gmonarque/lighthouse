<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import {
		Compass,
		User,
		Radio,
		Film,
		CheckCircle,
		ChevronRight,
		ChevronLeft,
		Key,
		Upload,
		RefreshCw,
		Loader2
	} from 'lucide-svelte';
	import { api } from '$lib/api/client';
	import { setupStatus, addToast } from '$lib/stores/app';

	let currentStep = 0;
	let isLoading = false;

	// Step 1: Identity
	let identityChoice: 'generate' | 'import' = 'generate';
	let importNsec = '';
	let generatedIdentity: { npub: string; nsec?: string } | null = null;

	// Step 2: Relays
	let selectedPreset = 'public';
	const relayPresets = {
		public: [
			{ url: 'wss://relay.damus.io', name: 'Damus' },
			{ url: 'wss://nos.lol', name: 'nos.lol' },
			{ url: 'wss://relay.primal.net', name: 'Primal' },
			{ url: 'wss://purplepag.es', name: 'Purple Pages' },
			{ url: 'wss://relay.nostr.band', name: 'Nostr Band' },
			{ url: 'wss://relay.snort.social', name: 'Snort' }
		],
		minimal: [
			{ url: 'wss://relay.damus.io', name: 'Damus' },
			{ url: 'wss://nos.lol', name: 'nos.lol' },
			{ url: 'wss://relay.primal.net', name: 'Primal' }
		],
		private: []
	};

	// Step 3: Enrichment
	let tmdbApiKey = '';
	let omdbApiKey = '';
	let skipEnrichment = false;

	const steps = [
		{ title: 'Welcome', icon: Compass },
		{ title: 'Identity', icon: User },
		{ title: 'Relays', icon: Radio },
		{ title: 'Metadata', icon: Film },
		{ title: 'Complete', icon: CheckCircle }
	];

	onMount(async () => {
		const status = await api.getSetupStatus();
		if (status.completed) {
			goto('/');
		}
	});

	async function nextStep() {
		if (currentStep === 1) {
			// Save identity
			isLoading = true;
			try {
				if (identityChoice === 'generate') {
					const result = await api.generateIdentity();
					generatedIdentity = result;
				} else if (importNsec) {
					await api.importIdentity(importNsec);
				}
			} catch (error) {
				addToast('error', 'Failed to set up identity');
				isLoading = false;
				return;
			}
			isLoading = false;
		}

		if (currentStep === 2) {
			// Save relays
			isLoading = true;
			try {
				const relays = relayPresets[selectedPreset as keyof typeof relayPresets];
				for (const relay of relays) {
					await api.addRelay(relay.url, relay.name, selectedPreset, true);
				}
			} catch (error) {
				// Relays might already exist, continue
			}
			isLoading = false;
		}

		if (currentStep === 3) {
			// Save API keys
			if (!skipEnrichment && (tmdbApiKey || omdbApiKey)) {
				isLoading = true;
				try {
					const updates: Record<string, string> = {};
					if (tmdbApiKey) updates['enrichment.tmdb_api_key'] = tmdbApiKey;
					if (omdbApiKey) updates['enrichment.omdb_api_key'] = omdbApiKey;
					await api.updateSettings(updates);
				} catch (error) {
					// Continue anyway
				}
				isLoading = false;
			}
		}

		if (currentStep === 4) {
			// Complete setup
			isLoading = true;
			try {
				await api.completeSetup();
				await api.startIndexer();
				setupStatus.set({ completed: true, has_identity: true, has_relays: true, has_tmdb_key: !!tmdbApiKey, enrichment_enabled: true });
				goto('/');
			} catch (error) {
				addToast('error', 'Failed to complete setup');
			}
			isLoading = false;
			return;
		}

		currentStep++;
	}

	function prevStep() {
		if (currentStep > 0) {
			currentStep--;
		}
	}
</script>

<div class="min-h-screen bg-surface-950 flex flex-col items-center justify-center p-4">
	<div class="w-full max-w-2xl">
		<!-- Progress -->
		<div class="flex items-center justify-center gap-2 mb-8">
			{#each steps as step, i}
				<div class="flex items-center">
					<div
						class="w-10 h-10 rounded-full flex items-center justify-center transition-colors {i <= currentStep ? 'bg-primary-600 text-white' : 'bg-surface-800 text-surface-500'}"
					>
						<svelte:component this={step.icon} class="w-5 h-5" />
					</div>
					{#if i < steps.length - 1}
						<div class="w-12 h-1 mx-1 {i < currentStep ? 'bg-primary-600' : 'bg-surface-800'}"></div>
					{/if}
				</div>
			{/each}
		</div>

		<div class="card">
			<!-- Step 0: Welcome -->
			{#if currentStep === 0}
				<div class="text-center py-8">
					<div class="w-20 h-20 bg-gradient-to-br from-primary-500 to-accent-500 rounded-2xl flex items-center justify-center mx-auto mb-6">
						<Compass class="w-10 h-10 text-white" />
					</div>
					<h1 class="text-3xl font-bold text-white mb-2">Welcome to Lighthouse</h1>
					<p class="text-surface-400 mb-6 max-w-md mx-auto">
						Your decentralized torrent indexer powered by Nostr. Let's set up your node in a few simple steps.
					</p>
					<p class="text-sm text-primary-400 font-medium">Your Node, Your Rules.</p>
				</div>
			{/if}

			<!-- Step 1: Identity -->
			{#if currentStep === 1}
				<div class="py-4">
					<h2 class="text-2xl font-bold text-white mb-2">Create Your Identity</h2>
					<p class="text-surface-400 mb-6">
						Your Nostr identity lets you follow trusted uploaders and build your Web of Trust.
					</p>

					<div class="space-y-4">
						<label class="flex items-start gap-3 p-4 rounded-lg border cursor-pointer transition-colors {identityChoice === 'generate' ? 'border-primary-500 bg-primary-900/20' : 'border-surface-700 hover:border-surface-600'}">
							<input
								type="radio"
								name="identity"
								value="generate"
								bind:group={identityChoice}
								class="mt-1"
							/>
							<div>
								<p class="font-medium text-surface-100">Generate New Identity</p>
								<p class="text-sm text-surface-400">Create a fresh Nostr keypair for this node</p>
							</div>
						</label>

						<label class="flex items-start gap-3 p-4 rounded-lg border cursor-pointer transition-colors {identityChoice === 'import' ? 'border-primary-500 bg-primary-900/20' : 'border-surface-700 hover:border-surface-600'}">
							<input
								type="radio"
								name="identity"
								value="import"
								bind:group={identityChoice}
								class="mt-1"
							/>
							<div class="flex-1">
								<p class="font-medium text-surface-100">Import Existing Identity</p>
								<p class="text-sm text-surface-400 mb-2">Use your existing Nostr nsec key</p>
								{#if identityChoice === 'import'}
									<input
										type="password"
										bind:value={importNsec}
										placeholder="nsec1..."
										class="input font-mono text-sm"
									/>
								{/if}
							</div>
						</label>
					</div>
				</div>
			{/if}

			<!-- Step 2: Relays -->
			{#if currentStep === 2}
				<div class="py-4">
					<h2 class="text-2xl font-bold text-white mb-2">Choose Relays</h2>
					<p class="text-surface-400 mb-6">
						Select which Nostr relays to connect to for discovering torrents.
					</p>

					<div class="space-y-4">
						<label class="flex items-start gap-3 p-4 rounded-lg border cursor-pointer transition-colors {selectedPreset === 'public' ? 'border-primary-500 bg-primary-900/20' : 'border-surface-700 hover:border-surface-600'}">
							<input
								type="radio"
								name="preset"
								value="public"
								bind:group={selectedPreset}
								class="mt-1"
							/>
							<div>
								<p class="font-medium text-surface-100">Public Relays (Recommended)</p>
								<p class="text-sm text-surface-400">Connect to popular public relays</p>
								<div class="flex flex-wrap gap-1 mt-2">
									{#each relayPresets.public as relay}
										<span class="badge badge-primary">{relay.name}</span>
									{/each}
								</div>
							</div>
						</label>

						<label class="flex items-start gap-3 p-4 rounded-lg border cursor-pointer transition-colors {selectedPreset === 'minimal' ? 'border-primary-500 bg-primary-900/20' : 'border-surface-700 hover:border-surface-600'}">
							<input
								type="radio"
								name="preset"
								value="minimal"
								bind:group={selectedPreset}
								class="mt-1"
							/>
							<div>
								<p class="font-medium text-surface-100">Minimal</p>
								<p class="text-sm text-surface-400">Just the essentials for low bandwidth</p>
								<div class="flex flex-wrap gap-1 mt-2">
									{#each relayPresets.minimal as relay}
										<span class="badge badge-primary">{relay.name}</span>
									{/each}
								</div>
							</div>
						</label>
					</div>
				</div>
			{/if}

			<!-- Step 3: Metadata -->
			{#if currentStep === 3}
				<div class="py-4">
					<h2 class="text-2xl font-bold text-white mb-2">Metadata Enrichment</h2>
					<p class="text-surface-400 mb-6">
						Add API keys to automatically fetch movie posters, descriptions, and ratings.
					</p>

					<div class="space-y-4">
						<div>
							<label class="label" for="setup-tmdb-key">TMDB API Key (Optional)</label>
							<input
								id="setup-tmdb-key"
								type="text"
								bind:value={tmdbApiKey}
								placeholder="Enter your TMDB API key"
								class="input"
							/>
							<a href="https://www.themoviedb.org/settings/api" target="_blank" class="text-xs text-primary-400 hover:underline">
								Get a free API key &rarr;
							</a>
						</div>

						<div>
							<label class="label" for="setup-omdb-key">OMDB API Key (Optional)</label>
							<input
								id="setup-omdb-key"
								type="text"
								bind:value={omdbApiKey}
								placeholder="Enter your OMDB API key"
								class="input"
							/>
							<a href="https://www.omdbapi.com/apikey.aspx" target="_blank" class="text-xs text-primary-400 hover:underline">
								Get a free API key &rarr;
							</a>
						</div>

						<label class="flex items-center gap-2 cursor-pointer">
							<input
								type="checkbox"
								bind:checked={skipEnrichment}
								class="rounded"
							/>
							<span class="text-surface-300">Skip for now (can add later in settings)</span>
						</label>
					</div>
				</div>
			{/if}

			<!-- Step 4: Complete -->
			{#if currentStep === 4}
				<div class="text-center py-8">
					<div class="w-20 h-20 bg-green-900/50 rounded-full flex items-center justify-center mx-auto mb-6">
						<CheckCircle class="w-10 h-10 text-green-400" />
					</div>
					<h2 class="text-2xl font-bold text-white mb-2">All Set!</h2>
					<p class="text-surface-400 mb-6 max-w-md mx-auto">
						Your Lighthouse node is ready. Click below to start indexing torrents from the Nostr network.
					</p>
					<div class="text-sm text-surface-500">
						The indexer will begin collecting content from your connected relays.
					</div>
				</div>
			{/if}

			<!-- Navigation -->
			<div class="flex items-center justify-between mt-8 pt-4 border-t border-surface-800">
				<button
					class="btn-secondary"
					onclick={prevStep}
					disabled={currentStep === 0 || isLoading}
				>
					<ChevronLeft class="w-4 h-4" />
					Back
				</button>

				<button
					class="btn-primary"
					onclick={nextStep}
					disabled={isLoading || (currentStep === 1 && identityChoice === 'import' && !importNsec)}
				>
					{#if isLoading}
						<Loader2 class="w-4 h-4 animate-spin" />
						Processing...
					{:else if currentStep === 4}
						Start Indexing
						<CheckCircle class="w-4 h-4" />
					{:else}
						Continue
						<ChevronRight class="w-4 h-4" />
					{/if}
				</button>
			</div>
		</div>
	</div>
</div>
