<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import {
		Home,
		Search,
		Upload,
		Shield,
		Radio,
		Settings,
		Activity,
		Compass,
		Menu,
		X,
		FileCheck,
		Flag,
		Key,
		Clock,
		BarChart3
	} from 'lucide-svelte';
	import { api } from '$lib/api/client';
	import { setupStatus, stats, indexerStatus, toasts, removeToast, addToast } from '$lib/stores/app';

	let mobileMenuOpen = false;

	const navItems = [
		{ href: '/', label: 'Dashboard', icon: Home },
		{ href: '/search', label: 'Search', icon: Search },
		{ href: '/publish', label: 'Publish', icon: Upload },
		{ href: '/trust', label: 'Trust', icon: Shield },
		{ href: '/curation', label: 'Curation', icon: FileCheck },
		{ href: '/reports', label: 'Reports', icon: Flag },
		{ href: '/relays', label: 'Relays', icon: Radio },
		{ href: '/settings', label: 'Settings', icon: Settings }
	];

	const adminItems = [
		{ href: '/admin/apikeys', label: 'API Keys', icon: Key },
		{ href: '/admin/activity', label: 'Activity', icon: Activity },
		{ href: '/admin/explorer', label: 'Explorer', icon: BarChart3 },
		{ href: '/admin/sla', label: 'SLA', icon: Clock }
	];

	onMount(async () => {
		try {
			// Check setup status
			const status = await api.getSetupStatus();
			setupStatus.set(status);

			// Redirect to setup if first run
			if (!status.completed && $page.url.pathname !== '/setup') {
				goto('/setup');
				return;
			}

			// Load initial data
			const [statsData, indexerData] = await Promise.all([
				api.getStats(),
				api.getIndexerStatus()
			]);

			stats.set(statsData);
			indexerStatus.set(indexerData);
		} catch (error) {
			console.error('Failed to load initial data:', error);
			addToast('error', 'Failed to connect to server');
		}
	});

	function isActive(href: string): boolean {
		if (href === '/') {
			return $page.url.pathname === '/';
		}
		return $page.url.pathname.startsWith(href);
	}
</script>

<svelte:head>
	<title>Lighthouse - Decentralized Torrent Indexer</title>
</svelte:head>

{#if $page.url.pathname === '/setup'}
	<!-- Setup wizard has its own layout -->
	<slot />
{:else}
	<div class="flex min-h-screen bg-surface-950">
		<!-- Sidebar -->
		<aside class="sidebar">
			<!-- Logo -->
			<div class="flex items-center gap-3 px-4 py-5 border-b border-surface-800">
				<div class="w-10 h-10 bg-gradient-to-br from-primary-500 to-accent-500 rounded-lg flex items-center justify-center">
					<Compass class="w-6 h-6 text-white" />
				</div>
				<div>
					<h1 class="text-lg font-bold text-white">Lighthouse</h1>
					<p class="text-xs text-surface-500">Your Node, Your Rules</p>
				</div>
			</div>

			<!-- Navigation -->
			<nav class="flex-1 px-3 py-4 space-y-1 overflow-y-auto">
				{#each navItems as item}
					<a
						href={item.href}
						class={isActive(item.href) ? 'nav-link-active' : 'nav-link'}
					>
						<svelte:component this={item.icon} class="w-5 h-5" />
						{item.label}
					</a>
				{/each}

				<!-- Admin Section -->
				<div class="pt-4 mt-4 border-t border-surface-800">
					<span class="px-3 text-xs font-medium text-surface-500 uppercase tracking-wider">Admin</span>
					<div class="mt-2 space-y-1">
						{#each adminItems as item}
							<a
								href={item.href}
								class={isActive(item.href) ? 'nav-link-active' : 'nav-link'}
							>
								<svelte:component this={item.icon} class="w-5 h-5" />
								{item.label}
							</a>
						{/each}
					</div>
				</div>
			</nav>

			<!-- Status footer -->
			<div class="px-4 py-4 border-t border-surface-800">
				<div class="flex items-center gap-2 text-xs">
					{#if $indexerStatus === null}
						<span class="w-2 h-2 rounded-full bg-yellow-500 animate-pulse"></span>
						<span class="text-surface-400">Loading...</span>
					{:else if $indexerStatus.running}
						<span class="w-2 h-2 rounded-full bg-green-500"></span>
						<span class="text-surface-400">Indexer Running</span>
					{:else}
						<span class="w-2 h-2 rounded-full bg-red-500"></span>
						<span class="text-surface-400">Indexer Stopped</span>
					{/if}
				</div>
				{#if $stats}
					<p class="text-xs text-surface-500 mt-1">
						{$stats.total_torrents.toLocaleString()} torrents indexed
					</p>
				{/if}
			</div>
		</aside>

		<!-- Mobile menu button -->
		<button
			class="fixed top-4 left-4 z-50 p-2 bg-surface-800 rounded-lg md:hidden"
			onclick={() => (mobileMenuOpen = !mobileMenuOpen)}
		>
			{#if mobileMenuOpen}
				<X class="w-6 h-6" />
			{:else}
				<Menu class="w-6 h-6" />
			{/if}
		</button>

		<!-- Main content -->
		<main class="page-container">
			<slot />
		</main>
	</div>

	<!-- Toast notifications -->
	<div class="fixed bottom-4 right-4 z-50 space-y-2">
		{#each $toasts as toast (toast.id)}
			<div
				class="toast-{toast.type} flex items-center gap-3 min-w-[300px] animate-slide-up"
				role="alert"
			>
				<span class="flex-1">{toast.message}</span>
				<button
					class="text-current opacity-70 hover:opacity-100"
					onclick={() => removeToast(toast.id)}
				>
					<X class="w-4 h-4" />
				</button>
			</div>
		{/each}
	</div>
{/if}
