package main

import (
	"fmt"
	"log"
	"net/http"
)

const statusHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Proxy Overview</title>
    <link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='%2318181b' stroke-width='2'><circle cx='12' cy='12' r='10'/><path d='M2 12h20M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z'/></svg>">
    <script src="https://cdn.tailwindcss.com"></script>
    <script>
        tailwind.config = {
            theme: {
                extend: {
                    fontFamily: { sans: ['Inter', 'system-ui', 'sans-serif'], mono: ['JetBrains Mono', 'monospace'] }
                }
            }
        }
    </script>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
    <style>
        [data-tooltip] { position: relative; }
        [data-tooltip]:hover::after {
            content: attr(data-tooltip);
            position: absolute;
            bottom: 100%;
            left: 50%;
            transform: translateX(-50%);
            padding: 6px 10px;
            background: #18181b;
            color: white;
            font-size: 12px;
            border-radius: 6px;
            white-space: nowrap;
            z-index: 50;
            margin-bottom: 4px;
        }
        [data-tooltip-right]:hover::after {
            content: attr(data-tooltip-right);
            position: absolute;
            bottom: 100%;
            right: 0;
            left: auto;
            transform: none;
            padding: 6px 10px;
            background: #18181b;
            color: white;
            font-size: 12px;
            border-radius: 6px;
            white-space: nowrap;
            z-index: 50;
            margin-bottom: 4px;
        }
    </style>
</head>
<body class="bg-zinc-50 text-zinc-900 min-h-screen">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <!-- Header -->
        <div class="mb-8">
            <h1 class="text-2xl font-semibold text-zinc-900">Proxy Overview</h1>
            <p class="text-sm text-zinc-500 mt-1"><span id="updated">Loading...</span> · <span id="service-count"></span></p>
        </div>

        <!-- Wildcard Domains -->
        <div id="wildcards" class="hidden mb-6">
            <div class="bg-white rounded-xl border border-zinc-200 p-4 shadow-sm">
                <div class="flex items-center gap-2 mb-2">
                    <svg class="w-4 h-4 text-amber-500" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z"/></svg>
                    <span class="text-sm font-medium text-zinc-700">Wildcard Certificates</span>
                    <span class="text-xs text-zinc-400">(subdomains not exposed in CT logs)</span>
                </div>
                <div id="wildcard-list" class="flex flex-wrap gap-2"></div>
            </div>
        </div>

        <!-- Filters -->
        <div class="flex flex-wrap items-center gap-4 mb-6">
            <div class="flex items-center gap-2">
                <span class="text-sm text-zinc-500">Search:</span>
                <input type="text" id="search" placeholder="Filter..." class="px-3 py-1.5 text-sm border border-zinc-200 rounded-lg bg-white focus:outline-none focus:ring-2 focus:ring-zinc-900 focus:border-transparent w-40">
            </div>
            <div class="flex items-center gap-2">
                <span class="text-sm text-zinc-500">Source:</span>
                <div class="inline-flex rounded-lg border border-zinc-200 bg-white p-1">
                    <button class="filter-btn px-3 py-1.5 text-sm font-medium rounded-md transition-colors" data-filter="all">All</button>
                    <button class="filter-btn px-3 py-1.5 text-sm font-medium rounded-md transition-colors" data-filter="managed">Managed</button>
                    <button class="filter-btn px-3 py-1.5 text-sm font-medium rounded-md transition-colors" data-filter="manual">Manual</button>
                </div>
            </div>
            <div class="flex items-center gap-2">
                <span class="text-sm text-zinc-500">Type:</span>
                <div class="inline-flex rounded-lg border border-zinc-200 bg-white p-1">
                    <button class="type-btn px-3 py-1.5 text-sm font-medium rounded-md transition-colors" data-type="all">All</button>
                    <button class="type-btn px-3 py-1.5 text-sm font-medium rounded-md transition-colors" data-type="external">External</button>
                    <button class="type-btn px-3 py-1.5 text-sm font-medium rounded-md transition-colors" data-type="internal">Internal</button>
                    <button class="type-btn px-3 py-1.5 text-sm font-medium rounded-md transition-colors" data-type="cloudflare">Cloudflare</button>
                </div>
            </div>
            <div class="flex items-center gap-2 relative">
                <span class="text-sm text-zinc-500">Options:</span>
                <button id="options-dropdown-btn" class="h-[42px] px-3 py-1.5 text-sm font-medium border border-zinc-200 rounded-lg bg-white transition-colors flex items-center gap-1">
                    <span id="options-count">All</span>
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/></svg>
                </button>
                <div id="options-dropdown" class="hidden absolute top-full left-0 mt-1 bg-white border border-zinc-200 rounded-lg shadow-lg z-50 min-w-48">
                    <div class="border-b border-zinc-200 p-2 space-y-1">
                        <div class="px-2 py-1 text-xs font-medium text-amber-600 uppercase tracking-wider">Issues</div>
                        <label class="flex items-center gap-2 px-2 py-1.5 hover:bg-amber-50 rounded cursor-pointer text-amber-700">
                            <input type="checkbox" class="issue-filter rounded border-amber-400 text-amber-600 focus:ring-amber-500" data-issue="unprotected">
                            <span class="text-sm">Unprotected</span>
                        </label>
                        <label class="flex items-center gap-2 px-2 py-1.5 hover:bg-amber-50 rounded cursor-pointer text-amber-700">
                            <input type="checkbox" class="issue-filter rounded border-amber-400 text-amber-600 focus:ring-amber-500" data-issue="no-seo">
                            <span class="text-sm">No SEO</span>
                        </label>
                        <label class="flex items-center gap-2 px-2 py-1.5 hover:bg-amber-50 rounded cursor-pointer text-amber-700">
                            <input type="checkbox" class="issue-filter rounded border-amber-400 text-amber-600 focus:ring-amber-500" data-issue="no-www">
                            <span class="text-sm">No WWW redirect</span>
                        </label>
                    </div>
                    <div class="p-2 space-y-1">
                        <div class="px-2 py-1 text-xs font-medium text-zinc-400 uppercase tracking-wider">Options</div>
                        <label class="flex items-center gap-2 px-2 py-1.5 hover:bg-zinc-50 rounded cursor-pointer">
                            <input type="checkbox" class="opt-filter rounded" data-opt="logging" data-field="logging">
                            <span class="text-sm">Logging</span>
                        </label>
                        <label class="flex items-center gap-2 px-2 py-1.5 hover:bg-zinc-50 rounded cursor-pointer">
                            <input type="checkbox" class="opt-filter rounded" data-opt="tls" data-field="tls">
                            <span class="text-sm">TLS</span>
                        </label>
                        <label class="flex items-center gap-2 px-2 py-1.5 hover:bg-zinc-50 rounded cursor-pointer">
                            <input type="checkbox" class="opt-filter rounded" data-opt="compression" data-field="compression">
                            <span class="text-sm">Compression</span>
                        </label>
                        <label class="flex items-center gap-2 px-2 py-1.5 hover:bg-zinc-50 rounded cursor-pointer">
                            <input type="checkbox" class="opt-filter rounded" data-opt="auth" data-field="auth">
                            <span class="text-sm">Auth</span>
                        </label>
                        <label class="flex items-center gap-2 px-2 py-1.5 hover:bg-zinc-50 rounded cursor-pointer">
                            <input type="checkbox" class="opt-filter rounded" data-opt="seo" data-field="seo">
                            <span class="text-sm">SEO indexable</span>
                        </label>
                        <label class="flex items-center gap-2 px-2 py-1.5 hover:bg-zinc-50 rounded cursor-pointer">
                            <input type="checkbox" class="opt-filter rounded" data-opt="wwwRedirect" data-field="wwwRedirect">
                            <span class="text-sm">WWW redirect</span>
                        </label>
                        <label class="flex items-center gap-2 px-2 py-1.5 hover:bg-zinc-50 rounded cursor-pointer">
                            <input type="checkbox" class="opt-filter rounded" data-opt="wordpress" data-field="wordpress">
                            <span class="text-sm">WordPress</span>
                        </label>
                        <label class="flex items-center gap-2 px-2 py-1.5 hover:bg-zinc-50 rounded cursor-pointer">
                            <input type="checkbox" class="opt-filter rounded" data-opt="allowlist" data-field="allowlist">
                            <span class="text-sm">Allowlist</span>
                        </label>
                        <label class="flex items-center gap-2 px-2 py-1.5 hover:bg-zinc-50 rounded cursor-pointer">
                            <input type="checkbox" class="opt-filter rounded" data-opt="trustedProxies" data-field="trustedProxies">
                            <span class="text-sm">Trusted proxies</span>
                        </label>
                    </div>
                    <div class="border-t border-zinc-200 p-2">
                        <button id="clear-options" class="w-full px-2 py-1.5 text-sm text-zinc-500 hover:text-zinc-700 hover:bg-zinc-50 rounded transition-colors">Clear filters</button>
                    </div>
                </div>
            </div>
        </div>

        <!-- Table -->
        <div class="bg-white rounded-xl border border-zinc-200 overflow-hidden shadow-sm">
            <div class="overflow-x-auto">
                <table class="w-full">
                    <thead>
                        <tr class="border-b border-zinc-200 bg-zinc-50/50">
                            <th class="text-left text-xs font-medium text-zinc-500 uppercase tracking-wider px-4 py-3">Domain</th>
                            <th class="text-left text-xs font-medium text-zinc-500 uppercase tracking-wider px-4 py-3">Type</th>
                            <th class="text-left text-xs font-medium text-zinc-500 uppercase tracking-wider px-4 py-3">Allowlist</th>
                            <th class="text-left text-xs font-medium text-zinc-500 uppercase tracking-wider px-4 py-3">Options</th>
                            <th id="config-header" class="text-center text-xs font-medium text-zinc-500 uppercase tracking-wider px-4 py-3 w-20">Config</th>
                        </tr>
                    </thead>
                    <tbody id="services" class="divide-y divide-zinc-100"></tbody>
                </table>
            </div>
        </div>

        <!-- Footer -->
        <div class="mt-6 text-center">
            <a href="/api/status" class="text-sm text-zinc-400 hover:text-zinc-600 transition-colors">JSON API</a>
        </div>
    </div>

    <script>
        let currentFilter = 'all';
        let currentType = 'all';
        let searchQuery = '';
        let allServices = [];
        let codeEditorUrl = '';
        let optionFilters = {}; // { fieldName: true } for active filters
        let issueFilters = {}; // { issueName: true } for active issue filters

        // Icons as inline SVGs
        const icons = {
            log: '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/></svg>',
            tls: '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/></svg>',
            gzip: '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7v10c0 2 1 3 3 3h10c2 0 3-1 3-3V7c0-2-1-3-3-3H7C5 4 4 5 4 7zm4 0h8m-8 4h8m-8 4h4"/></svg>',
            header: '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h7"/></svg>',
            security: '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"/></svg>',
            perf: '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"/></svg>',
            auth: '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"/></svg>',
            seo: '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/></svg>',
            www: '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 9l3 3m0 0l-3 3m3-3H8m13 0a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>',
            wp: '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 5a2 2 0 012-2h14a2 2 0 012 2v14a2 2 0 01-2 2H5a2 2 0 01-2-2V5zm4 4h10M7 12h10m-7 4h4"/></svg>',
            proxy: '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4"/></svg>',
            external: '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"/></svg>',
            managed: '<svg class="w-3 h-3" fill="currentColor" viewBox="0 0 24 24"><circle cx="12" cy="12" r="8"/></svg>',
            manual: '<svg class="w-3 h-3" fill="none" stroke="currentColor" stroke-width="3" viewBox="0 0 24 24"><circle cx="12" cy="12" r="8"/></svg>',
            copy: '<svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"/></svg>',
            check: '<svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/></svg>'
        };

        function copyToClipboard(text, btn) {
            navigator.clipboard.writeText(text).then(() => {
                const original = btn.innerHTML;
                btn.innerHTML = icons.check;
                btn.classList.add('text-emerald-500');
                setTimeout(() => {
                    btn.innerHTML = original;
                    btn.classList.remove('text-emerald-500');
                }, 1000);
            });
        }

        const optionInfo = {
            log: 'Request logging',
            gzip: 'Compression',
            header: 'Headers',
            security: 'Security',
            perf: 'Performance',
            auth: 'Auth',
            seo: 'SEO indexable',
            www: 'www→ redirect',
            wp: 'WordPress',
            proxy: 'Trusted proxies'
        };

        function getTlsTooltip(dnsProvider) {
            if (!dnsProvider || dnsProvider === 'http') return 'TLS (ACME)';
            if (dnsProvider === 'hetzner') return 'TLS (Hetzner)';
            return 'TLS (Cloudflare)';
        }

        function loadFilterFromHash() {
            const hash = window.location.hash.slice(1);
            const parts = hash.split(',');
            if (parts[0] && ['all', 'managed', 'manual'].includes(parts[0])) {
                currentFilter = parts[0];
            }
            if (parts[1] && ['all', 'external', 'internal', 'cloudflare'].includes(parts[1])) {
                currentType = parts[1];
            }
            updateFilterButtons();
            updateTypeButtons();
        }

        function updateFilterButtons() {
            document.querySelectorAll('.filter-btn').forEach(b => {
                const isActive = b.dataset.filter === currentFilter;
                b.classList.toggle('bg-zinc-900', isActive);
                b.classList.toggle('text-white', isActive);
                b.classList.toggle('text-zinc-600', !isActive);
                b.classList.toggle('hover:bg-zinc-100', !isActive);
            });
        }

        function updateTypeButtons() {
            document.querySelectorAll('.type-btn').forEach(b => {
                const isActive = b.dataset.type === currentType;
                b.classList.toggle('bg-zinc-900', isActive);
                b.classList.toggle('text-white', isActive);
                b.classList.toggle('text-zinc-600', !isActive);
                b.classList.toggle('hover:bg-zinc-100', !isActive);
            });
        }

        function domainLinks(domains) {
            if (!domains || domains.length === 0) return '<span class="text-zinc-400">—</span>';
            return domains.map(d =>
                '<div class="flex items-center gap-1.5 leading-relaxed group">' +
                '<a href="https://' + d + '" target="_blank" class="text-zinc-900 hover:text-blue-600 transition-colors">' + d + '</a>' +
                '<button onclick="copyToClipboard(\'' + d + '\', this)" class="text-zinc-300 hover:text-zinc-500 opacity-0 group-hover:opacity-100 transition-opacity">' + icons.copy + '</button>' +
                '</div>'
            ).join('');
        }

        function typeLabel(type, managed) {
            const colors = {
                external: 'text-blue-600',
                internal: 'text-violet-600',
                cloudflare: 'text-orange-500'
            };
            const dot = managed
                ? '<span class="text-emerald-500 mr-1.5" data-tooltip="Managed">' + icons.managed + '</span>'
                : '<span class="text-zinc-400 mr-1.5" data-tooltip="Manual">' + icons.manual + '</span>';
            return dot + '<span class="' + (colors[type] || 'text-zinc-600') + ' font-medium">' + type + '</span>';
        }

        function optionIcons(svc) {
            // Build auth tooltip with details
            let authTooltip = 'Auth';
            if (svc.auth) {
                const parts = [];
                if (svc.authPaths && svc.authPaths.length > 0) {
                    parts.push('Paths: ' + svc.authPaths.join(', '));
                } else if (svc.authExcept && svc.authExcept.length > 0) {
                    parts.push('Except: ' + svc.authExcept.join(', '));
                } else {
                    parts.push('Full site');
                }
                if (svc.authGroups && svc.authGroups.length > 0) {
                    parts.push('Groups: ' + svc.authGroups.join(', '));
                }
                if (svc.authUrl) {
                    parts.push('Server: ' + svc.authUrl);
                } else {
                    parts.push('Local tinyauth');
                }
                authTooltip = 'Auth ✓ (' + parts.join(' | ') + ')';
            } else {
                authTooltip = 'Auth ✗';
            }

            // Build trusted proxies tooltip with details
            const hasTrustedProxies = svc.trustedProxies && svc.trustedProxies.length > 0;
            let proxyTooltip = 'Trusted proxies';
            if (hasTrustedProxies) {
                proxyTooltip = 'Trusted proxies ✓ (' + svc.trustedProxies.join(', ') + ')';
            } else {
                proxyTooltip = 'Trusted proxies ✗';
            }

            // TLS is enabled if dnsProvider is set and not 'http'
            const hasTls = svc.dnsProvider && svc.dnsProvider !== 'http';

            const opts = [
                { key: 'log', enabled: svc.logging, tooltip: optionInfo.log },
                { key: 'tls', enabled: hasTls, tooltip: getTlsTooltip(svc.dnsProvider) },
                { key: 'gzip', enabled: svc.compression, tooltip: optionInfo.gzip },
                { key: 'header', enabled: svc.header, tooltip: optionInfo.header },
                { key: 'security', enabled: svc.security, tooltip: optionInfo.security },
                { key: 'perf', enabled: svc.performance, tooltip: optionInfo.perf },
                { key: 'auth', enabled: svc.auth, tooltip: authTooltip },
                { key: 'seo', enabled: svc.seo, tooltip: optionInfo.seo },
                { key: 'www', enabled: svc.wwwRedirect, tooltip: optionInfo.www },
                { key: 'wp', enabled: svc.wordpress, tooltip: optionInfo.wp },
                { key: 'proxy', enabled: hasTrustedProxies, tooltip: proxyTooltip }
            ];
            return opts.map(o => {
                const cls = o.enabled ? 'text-emerald-500' : 'text-zinc-300';
                const tooltip = o.tooltip + (o.enabled ? ' ✓' : ' ✗');
                return '<span class="' + cls + '" data-tooltip="' + tooltip + '">' + icons[o.key] + '</span>';
            }).join('');
        }

        function configLink(svc) {
            if (!codeEditorUrl || !svc.configPath) {
                return '<span class="text-zinc-300">—</span>';
            }
            const folder = svc.configPath.substring(0, svc.configPath.lastIndexOf('/'));
            return '<a href="' + codeEditorUrl + folder + '" target="_blank" class="text-zinc-400 hover:text-zinc-600 transition-colors" data-tooltip-right="Open in editor">' + icons.external + '</a>';
        }

        function getFirstDomain(svc) {
            return (svc.domains && svc.domains[0]) || '';
        }

        function checkIssue(svc, issueType) {
            switch (issueType) {
                case 'unprotected':
                    // External + no auth + no allowlist = unprotected
                    return svc.type === 'external' && !svc.auth && (!svc.allowlist || svc.allowlist.length === 0);
                case 'no-seo':
                    // External + SEO disabled = might be unintentional
                    return svc.type === 'external' && !svc.seo;
                case 'no-www':
                    // SEO enabled but no WWW redirect = duplicate content risk
                    return svc.seo && !svc.wwwRedirect;
                default:
                    return false;
            }
        }

        function renderServices() {
            const tbody = document.getElementById('services');
            let services = allServices;

            if (currentFilter === 'managed') {
                services = services.filter(s => s.managed);
            } else if (currentFilter === 'manual') {
                services = services.filter(s => !s.managed);
            }

            if (currentType !== 'all') {
                services = services.filter(s => s.type === currentType);
            }

            if (searchQuery) {
                const q = searchQuery.toLowerCase();
                services = services.filter(s =>
                    (s.domains || []).some(d => d.toLowerCase().includes(q)) ||
                    (s.container || '').toLowerCase().includes(q) ||
                    (s.network || '').toLowerCase().includes(q) ||
                    (s.type || '').toLowerCase().includes(q) ||
                    (s.allowlist || []).some(a => a.toLowerCase().includes(q))
                );
            }

            // Apply issue filters (OR logic - service matches ANY selected issue)
            const activeIssues = Object.keys(issueFilters).filter(k => issueFilters[k]);
            if (activeIssues.length > 0) {
                services = services.filter(s => activeIssues.some(issue => checkIssue(s, issue)));
            }

            // Apply option filters (AND logic - service must have ALL selected options)
            const activeFilters = Object.keys(optionFilters).filter(k => optionFilters[k]);
            if (activeFilters.length > 0) {
                services = services.filter(s => {
                    return activeFilters.every(field => {
                        if (field === 'allowlist') {
                            return s.allowlist && s.allowlist.length > 0;
                        }
                        if (field === 'trustedProxies') {
                            return s.trustedProxies && s.trustedProxies.length > 0;
                        }
                        return s[field] === true;
                    });
                });
            }

            services = services.slice().sort((a, b) => getFirstDomain(a).localeCompare(getFirstDomain(b)));

            const showConfig = !!codeEditorUrl;
            const colspan = showConfig ? 5 : 4;

            if (services.length === 0) {
                tbody.innerHTML = '<tr><td colspan="' + colspan + '" class="px-4 py-8 text-center text-zinc-400">No services configured</td></tr>';
            } else {
                tbody.innerHTML = services.map(svc => ` + "`" + `
                    <tr class="hover:bg-zinc-50/50 transition-colors">
                        <td class="px-4 py-3 font-mono text-sm">${domainLinks(svc.domains)}</td>
                        <td class="px-4 py-3 text-sm"><div class="flex items-center">${typeLabel(svc.type, svc.managed)}</div></td>
                        <td class="px-4 py-3 font-mono text-sm text-zinc-500">${(svc.allowlist || []).map(a => '<div class="flex items-center gap-1.5 leading-relaxed group"><span>' + a + '</span><button onclick="copyToClipboard(\'' + a + '\', this)" class="text-zinc-300 hover:text-zinc-500 opacity-0 group-hover:opacity-100 transition-opacity">' + icons.copy + '</button></div>').join('') || '<span class="text-zinc-300">—</span>'}</td>
                        <td class="px-4 py-3"><div class="flex items-center gap-1">${optionIcons(svc)}</div></td>
                        ${showConfig ? '<td class="config-cell px-4 py-3 text-center">' + configLink(svc) + '</td>' : ''}
                    </tr>
                ` + "`" + `).join('');
            }

            // Hide/show config column header
            document.getElementById('config-header').style.display = showConfig ? '' : 'none';

            // Update service count
            const countEl = document.getElementById('service-count');
            if (services.length === allServices.length) {
                countEl.textContent = allServices.length + ' services';
            } else {
                countEl.textContent = services.length + ' of ' + allServices.length + ' services';
            }
        }

        function renderWildcards(domains) {
            const container = document.getElementById('wildcards');
            const list = document.getElementById('wildcard-list');
            if (!domains || domains.length === 0) {
                container.classList.add('hidden');
                return;
            }
            container.classList.remove('hidden');
            list.innerHTML = domains.map(d =>
                '<span class="inline-flex items-center px-3 py-1 rounded-full text-sm font-mono bg-amber-50 text-amber-700 border border-amber-200">*.' + d + '</span>'
            ).join('');
        }

        async function loadStatus() {
            try {
                const res = await fetch('/api/status');
                const data = await res.json();
                document.getElementById('updated').textContent = 'Updated ' + new Date(data.updated).toLocaleString();
                // Filter out wildcard configs (they're shown separately)
                allServices = (data.services || []).filter(s => !s.configPath || !s.configPath.includes('/wildcard.'));
                codeEditorUrl = data.codeEditorUrl || '';
                // Set page title from statusDomain
                if (data.statusDomain) {
                    const firstDomain = data.statusDomain.split(',')[0].trim();
                    document.title = firstDomain + ' - Proxy Overview';
                }
                renderWildcards(data.wildcardDomains);
                renderServices();
            } catch (err) {
                document.getElementById('updated').textContent = 'Error: ' + err.message;
            }
        }

        function updateHash() {
            const parts = [];
            if (currentFilter !== 'all') parts.push(currentFilter);
            else if (currentType !== 'all') parts.push('all');
            if (currentType !== 'all') parts.push(currentType);
            window.location.hash = parts.join(',');
        }

        document.querySelectorAll('.filter-btn').forEach(btn => {
            btn.addEventListener('click', () => {
                currentFilter = btn.dataset.filter;
                updateHash();
                updateFilterButtons();
                renderServices();
            });
        });

        document.querySelectorAll('.type-btn').forEach(btn => {
            btn.addEventListener('click', () => {
                currentType = btn.dataset.type;
                updateHash();
                updateTypeButtons();
                renderServices();
            });
        });

        document.getElementById('search').addEventListener('input', (e) => {
            searchQuery = e.target.value;
            renderServices();
        });

        // Options dropdown
        const dropdownBtn = document.getElementById('options-dropdown-btn');
        const dropdown = document.getElementById('options-dropdown');
        const optionsCount = document.getElementById('options-count');

        dropdownBtn.addEventListener('click', (e) => {
            e.stopPropagation();
            dropdown.classList.toggle('hidden');
        });

        document.addEventListener('click', (e) => {
            if (!dropdown.contains(e.target) && e.target !== dropdownBtn) {
                dropdown.classList.add('hidden');
            }
        });

        function updateOptionsCount() {
            const optCount = Object.keys(optionFilters).filter(k => optionFilters[k]).length;
            const issueCount = Object.keys(issueFilters).filter(k => issueFilters[k]).length;

            if (issueCount > 0 && optCount > 0) {
                optionsCount.textContent = issueCount + ' issues, ' + optCount + ' opts';
                dropdownBtn.classList.add('bg-amber-500', 'text-white', 'border-amber-500');
                dropdownBtn.classList.remove('bg-zinc-900', 'border-zinc-900');
            } else if (issueCount > 0) {
                optionsCount.textContent = issueCount + ' issue' + (issueCount > 1 ? 's' : '');
                dropdownBtn.classList.add('bg-amber-500', 'text-white', 'border-amber-500');
                dropdownBtn.classList.remove('bg-zinc-900', 'border-zinc-900');
            } else if (optCount > 0) {
                optionsCount.textContent = optCount + ' selected';
                dropdownBtn.classList.add('bg-zinc-900', 'text-white', 'border-zinc-900');
                dropdownBtn.classList.remove('bg-amber-500', 'border-amber-500');
            } else {
                optionsCount.textContent = 'All';
                dropdownBtn.classList.remove('bg-zinc-900', 'text-white', 'border-zinc-900', 'bg-amber-500', 'border-amber-500');
            }
        }

        document.querySelectorAll('.opt-filter').forEach(cb => {
            cb.addEventListener('change', () => {
                const field = cb.dataset.field;
                if (cb.checked) {
                    optionFilters[field] = true;
                } else {
                    delete optionFilters[field];
                }
                updateOptionsCount();
                renderServices();
            });
        });

        // Issue filters
        document.querySelectorAll('.issue-filter').forEach(cb => {
            cb.addEventListener('change', () => {
                const issue = cb.dataset.issue;
                if (cb.checked) {
                    issueFilters[issue] = true;
                } else {
                    delete issueFilters[issue];
                }
                updateOptionsCount();
                renderServices();
            });
        });

        document.getElementById('clear-options').addEventListener('click', () => {
            optionFilters = {};
            issueFilters = {};
            document.querySelectorAll('.opt-filter').forEach(cb => cb.checked = false);
            document.querySelectorAll('.issue-filter').forEach(cb => cb.checked = false);
            updateOptionsCount();
            renderServices();
        });

        loadFilterFromHash();
        loadStatus();
        setInterval(loadStatus, 5000);
    </script>
</body>
</html>`

// StatusServer serves the status page and API
type StatusServer struct {
	statusMgr *StatusManager
	caddyMgr  *CaddyManager
	port      int
}

// NewStatusServer creates a new status server
func NewStatusServer(statusMgr *StatusManager, caddyMgr *CaddyManager, port int) *StatusServer {
	return &StatusServer{
		statusMgr: statusMgr,
		caddyMgr:  caddyMgr,
		port:      port,
	}
}

// Start starts the HTTP server
func (s *StatusServer) Start() {
	http.HandleFunc("/", s.handleHTML)
	http.HandleFunc("/api/status", s.handleJSON)

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("[watcher] Status server listening on %s", addr)

	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Printf("[watcher] Status server error: %v", err)
		}
	}()
}

func (s *StatusServer) handleHTML(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(statusHTML))
}

func (s *StatusServer) handleJSON(w http.ResponseWriter, r *http.Request) {
	// Refresh from disk to catch manual changes
	if s.caddyMgr != nil {
		s.statusMgr.Update(s.caddyMgr.ListConfigs())
	}

	data, err := s.statusMgr.GetJSON()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"failed to get status"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}
