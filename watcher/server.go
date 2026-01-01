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
    <title>Caddy Watcher</title>
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
    </style>
</head>
<body>
    <main class="container">
        <header>
            <h1>Caddy Watcher</h1>
            <p class="updated">Loading...</p>
        </header>

        <div class="overflow-auto">
            <table id="services" role="grid">
                <thead>
                    <tr>
                        <th scope="col">Network</th>
                        <th scope="col">Type</th>
                        <th scope="col">Domains</th>
                        <th scope="col">Allowlist</th>
                        <th scope="col">Options</th>
                        <th scope="col">Source</th>
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

        async function loadStatus() {
            try {
                const res = await fetch('/api/status');
                const data = await res.json();

                document.querySelector('.updated').textContent = new Date(data.updated).toLocaleString();

                const tbody = document.querySelector('#services tbody');
                if (data.services.length === 0) {
                    tbody.innerHTML = '<tr><td colspan="6">No services configured</td></tr>';
                } else {
                    tbody.innerHTML = data.services.map(svc => ` + "`" + `
                        <tr>
                            <td class="mono">${svc.network}</td>
                            <td><span class="badge badge-${svc.type}">${svc.type}</span></td>
                            <td class="mono">${domainLinks(svc.domains)}</td>
                            <td class="mono">${(svc.allowlist || []).join(', ') || '-'}</td>
                            <td>${optionBadge('log', svc.logging)}${optionBadge('dns', svc.tls)}${optionBadge('gzip', svc.compression)}${optionBadge('security', svc.header)}</td>
                            <td><span class="badge ${svc.managed ? 'badge-managed' : 'badge-manual'}">${svc.managed ? 'managed' : 'manual'}</span></td>
                        </tr>
                    ` + "`" + `).join('');
                }
            } catch (err) {
                document.querySelector('.updated').textContent = 'Error: ' + err.message;
            }
        }

        loadStatus();
        setInterval(loadStatus, 5000);
    </script>
</body>
</html>`

// StatusServer serves the status page and API
type StatusServer struct {
	statusMgr *StatusManager
	port      int
}

// NewStatusServer creates a new status server
func NewStatusServer(statusMgr *StatusManager, port int) *StatusServer {
	return &StatusServer{
		statusMgr: statusMgr,
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
