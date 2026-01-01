package main

import (
	"fmt"
	"log"
	"net/http"
)

const statusHTML = `<!DOCTYPE html>
<html lang="en" data-theme="light">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Proxy Overview</title>
    <link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>🌐</text></svg>">
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">
    <style>
        :root { --pico-font-size: 16px; }
        .badge {
            display: inline-block;
            padding: 0.2rem 0.5rem;
            font-size: 0.7rem;
            font-weight: 600;
            border-radius: var(--pico-border-radius);
            text-transform: uppercase;
            margin-right: 0.25rem;
            cursor: help;
        }
        .badge-managed { background: var(--pico-primary); color: var(--pico-primary-inverse); }
        .badge-manual { background: var(--pico-secondary); color: var(--pico-secondary-inverse); }
        .badge-external { background: #2563eb; color: white; }
        .badge-internal { background: #7c3aed; color: white; }
        .badge-cloudflare { background: #f97316; color: white; }
        .badge-on { background: #22c55e; color: white; }
        .badge-off { background: #ef4444; color: white; }
        .mono { font-family: var(--pico-font-family-monospace); font-size: 0.85rem; }
        header { margin-bottom: 1.5rem; }
        h1 { margin-bottom: 0.25rem; }
        .updated { color: var(--pico-muted-color); font-size: 0.8rem; margin: 0.5rem 0 0 0; }
        table tbody tr:nth-child(odd) td { background: var(--pico-background-color) !important; }
        table tbody tr:nth-child(even) td { background: #f1f5f9 !important; }
        .filters { margin-bottom: 1rem; display: flex; gap: 0.5rem; align-items: center; }
        .filters span { font-size: 0.9rem; color: var(--pico-muted-color); }
        .filter-btn { padding: 0.3rem 0.6rem; font-size: 0.8rem; cursor: pointer; border-radius: var(--pico-border-radius); border: 1px solid var(--pico-muted-border-color) !important; background: var(--pico-background-color) !important; color: var(--pico-color) !important; transition: all 0.2s; }
        .filter-btn:hover { border-color: var(--pico-primary) !important; }
        .filter-btn.active { background: var(--pico-primary) !important; color: var(--pico-primary-inverse) !important; border-color: var(--pico-primary) !important; }
    </style>
</head>
<body>
    <main class="container">
        <header>
            <h1>Proxy Overview</h1>
            <p class="updated">Loading...</p>
        </header>

        <div class="filters">
            <span>Filter:</span>
            <button class="filter-btn active" data-filter="all">All</button>
            <button class="filter-btn" data-filter="managed">Managed</button>
            <button class="filter-btn" data-filter="manual">Manual</button>
        </div>

        <div class="overflow-auto">
            <table id="services" role="grid">
                <thead>
                    <tr>
                        <th scope="col">Domains</th>
                        <th scope="col">Type</th>
                        <th scope="col">Allowlist</th>
                        <th scope="col">Options</th>
                        <th scope="col">Source</th>
                        <th scope="col">Config</th>
                    </tr>
                </thead>
                <tbody></tbody>
            </table>
        </div>

        <footer>
            <small><a href="/api/status">JSON API</a></small>
        </footer>
    </main>
    <script>
        const options = {
            log: { label: 'log', desc: 'Request logging', env: 'CADDY_LOGGING' },
            dns: { label: 'dns', desc: 'TLS via Cloudflare DNS challenge', env: 'CADDY_TLS' },
            gzip: { label: 'gzip', desc: 'zstd/gzip compression', env: 'CADDY_COMPRESSION' },
            security: { label: 'security', desc: 'Security headers', env: 'CADDY_HEADER' }
        };

        let currentFilter = 'all';
        let allServices = [];
        let codeEditorUrl = '';

        // Restore filter from URL hash
        function loadFilterFromHash() {
            const hash = window.location.hash.slice(1);
            if (['all', 'managed', 'manual'].includes(hash)) {
                currentFilter = hash;
                document.querySelectorAll('.filter-btn').forEach(b => {
                    b.classList.toggle('active', b.dataset.filter === currentFilter);
                });
            }
        }

        function optionBadge(key, enabled) {
            const opt = options[key];
            const state = enabled ? 'enabled' : 'disabled';
            const cls = enabled ? 'badge-on' : 'badge-off';
            return '<span class="badge ' + cls + '" title="' + opt.desc + ' (' + state + ')\n' + opt.env + '">' + opt.label + '</span>';
        }

        function domainLinks(domains) {
            if (!domains || domains.length === 0) return '-';
            return domains.map(d => '<a href="https://' + d + '" target="_blank" rel="noopener">' + d + '</a>').join(', ');
        }

        function getFirstDomain(svc) {
            return (svc.domains && svc.domains[0]) || '';
        }

        function configLink(svc) {
            const name = svc.container ? svc.container + '_' + svc.network : svc.network;
            if (codeEditorUrl && svc.configPath) {
                return '<a href="' + codeEditorUrl + svc.configPath + '" target="_blank" rel="noopener">' + name + '</a>';
            }
            return name;
        }

        function renderServices() {
            const tbody = document.querySelector('#services tbody');
            let services = allServices;

            // Apply filter
            if (currentFilter === 'managed') {
                services = services.filter(s => s.managed);
            } else if (currentFilter === 'manual') {
                services = services.filter(s => !s.managed);
            }

            // Sort by first domain A-Z
            services = services.slice().sort((a, b) => getFirstDomain(a).localeCompare(getFirstDomain(b)));

            if (services.length === 0) {
                tbody.innerHTML = '<tr><td colspan="6">No services configured</td></tr>';
            } else {
                tbody.innerHTML = services.map(svc => ` + "`" + `
                    <tr>
                        <td class="mono">${domainLinks(svc.domains)}</td>
                        <td><span class="badge badge-${svc.type}">${svc.type}</span></td>
                        <td class="mono">${(svc.allowlist || []).join(', ') || '-'}</td>
                        <td>${optionBadge('log', svc.logging)}${optionBadge('dns', svc.tls)}${optionBadge('gzip', svc.compression)}${optionBadge('security', svc.header)}</td>
                        <td><span class="badge ${svc.managed ? 'badge-managed' : 'badge-manual'}">${svc.managed ? 'managed' : 'manual'}</span></td>
                        <td class="mono">${configLink(svc)}</td>
                    </tr>
                ` + "`" + `).join('');
            }
        }

        async function loadStatus() {
            try {
                const res = await fetch('/api/status');
                const data = await res.json();

                document.querySelector('.updated').textContent = new Date(data.updated).toLocaleString();
                allServices = data.services || [];
                codeEditorUrl = data.codeEditorUrl || '';
                renderServices();
            } catch (err) {
                document.querySelector('.updated').textContent = 'Error: ' + err.message;
            }
        }

        // Filter button handlers
        document.querySelectorAll('.filter-btn').forEach(btn => {
            btn.addEventListener('click', () => {
                document.querySelectorAll('.filter-btn').forEach(b => b.classList.remove('active'));
                btn.classList.add('active');
                currentFilter = btn.dataset.filter;
                window.location.hash = currentFilter === 'all' ? '' : currentFilter;
                renderServices();
            });
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
