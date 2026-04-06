package scaffold

const tmplGoMod = `module {{.ModulePath}}

go 1.22

require (
	github.com/grafana/grafana-app-sdk v0.52.2
)
`

const tmplMain = `package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/grafana/grafana-app-sdk/quickstart/local"
)

func main() {
	dbPath := "{{.KindLower}}s.db"
	if v := os.Getenv("DB_PATH"); v != "" {
		dbPath = v
	}
	port := "{{.Port}}"
	if v := os.Getenv("PORT"); v != "" {
		port = v
	}

	srv, err := local.NewServer(dbPath)
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}
	defer srv.Close()

	srv.RegisterResource(local.GroupVersionResource{
		Group:    "{{.GroupName}}",
		Version:  "v1alpha1",
		Resource: "{{.KindPlural}}",
		Kind:     "{{.KindName}}",
	})

	addr := ":" + port
	fmt.Printf("\n  {{.KindName}} app running!\n")
	fmt.Printf("  API: http://localhost%s/apis/{{.GroupName}}/v1alpha1/namespaces/default/{{.KindPlural}}\n", addr)
	fmt.Printf("  Health: http://localhost%s/healthz\n\n", addr)

	handler := local.CORSMiddleware(srv)

	// Serve the frontend at /
	frontendDir := "plugin/src"
	if _, err := os.Stat(frontendDir); err == nil {
		fs := http.FileServer(http.Dir(frontendDir))
		mux := http.NewServeMux()
		mux.Handle("/apis/", handler)
		mux.Handle("/healthz", handler)
		mux.Handle("/", fs)
		handler = mux
	}

	log.Fatal(http.ListenAndServe(addr, handler))
}
`

const tmplKind = `package kinds

// {{.KindName}} defines the spec for the {{.KindName}} Kind.
//
// +kind:{{.KindName}}
// +group:{{.GroupName}}
// +version:v1alpha1
// +scope:Namespaced
type {{.KindName}}Spec struct {
	// Title is the display name.
	Title string ` + "`" + `json:"title" validate:"required" description:"Display title"` + "`" + `
	// Description is an optional longer description.
	Description string ` + "`" + `json:"description,omitempty" description:"Optional description"` + "`" + `
	// Value is a user-defined field.
	Value string ` + "`" + `json:"value,omitempty" description:"Custom value"` + "`" + `
}
`

const tmplIndexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{.KindName}} Manager</title>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body { font-family: system-ui, -apple-system, sans-serif; background: #fafafa; color: #1a1a1a; }
    .container { max-width: 720px; margin: 2rem auto; padding: 0 1rem; }
    h1 { font-size: 1.5rem; border-bottom: 2px solid #f60; padding-bottom: 8px; margin-bottom: 1.5rem; }
    .error { color: #c00; margin-bottom: 1rem; }
    form { display: flex; flex-direction: column; gap: 8px; margin-bottom: 1.5rem; }
    input { padding: 8px; font-size: 14px; border: 1px solid #ccc; border-radius: 4px; }
    .btn-row { display: flex; gap: 8px; }
    .btn { padding: 8px 16px; border: none; border-radius: 4px; cursor: pointer; font-size: 14px; }
    .btn-primary { background: #f60; color: #fff; }
    .btn-primary:hover { background: #e55; }
    .btn-cancel { background: #eee; }
    table { width: 100%; border-collapse: collapse; }
    th, td { padding: 8px; text-align: left; }
    th { border-bottom: 1px solid #ddd; font-weight: 600; }
    td { border-bottom: 1px solid #eee; }
    .mono { font-family: monospace; font-size: 13px; }
    .actions button { margin-right: 4px; background: none; border: 1px solid #ccc; border-radius: 3px;
                      padding: 4px 8px; cursor: pointer; font-size: 12px; }
    .actions .del { color: #c00; border-color: #c00; }
    .empty { padding: 24px; text-align: center; color: #999; }
  </style>
</head>
<body>
  <div class="container">
    <h1>{{.KindName}} Manager</h1>
    <div class="error" id="error" style="display:none"></div>
    <form id="form">
      <input id="f-title" placeholder="Title (required)" required>
      <input id="f-desc" placeholder="Description">
      <input id="f-value" placeholder="Value">
      <div class="btn-row">
        <button type="submit" class="btn btn-primary" id="btn-save">Create</button>
        <button type="button" class="btn btn-cancel" id="btn-cancel" style="display:none" onclick="cancelEdit()">Cancel</button>
      </div>
    </form>
    <table>
      <thead><tr><th>Name</th><th>Title</th><th>Description</th><th>Value</th><th>Actions</th></tr></thead>
      <tbody id="items"></tbody>
    </table>
  </div>
  <script>
    const API = '/apis/{{.GroupName}}/v1alpha1/namespaces/default/{{.KindPlural}}';
    let editing = null;

    async function load() {
      try {
        const res = await fetch(API);
        const data = await res.json();
        const items = data.items || [];
        const tbody = document.getElementById('items');
        if (items.length === 0) {
          tbody.innerHTML = '<tr><td colspan="5" class="empty">No items yet. Create one above!</td></tr>';
          return;
        }
        tbody.innerHTML = items.map(i => '<tr>' +
          '<td class="mono">' + esc(i.metadata.name) + '</td>' +
          '<td>' + esc(i.spec.title || '') + '</td>' +
          '<td>' + esc(i.spec.description || '') + '</td>' +
          '<td>' + esc(i.spec.value || '') + '</td>' +
          '<td class="actions">' +
            '<button onclick="editItem(\'' + esc(i.metadata.name) + '\',\'' + esc(i.spec.title||'') + '\',\'' + esc(i.spec.description||'') + '\',\'' + esc(i.spec.value||'') + '\')">Edit</button>' +
            '<button class="del" onclick="del(\'' + esc(i.metadata.name) + '\')">Delete</button>' +
          '</td></tr>'
        ).join('');
        hideError();
      } catch(e) { showError('Failed to load: ' + e.message); }
    }

    document.getElementById('form').onsubmit = async function(e) {
      e.preventDefault();
      const title = document.getElementById('f-title').value;
      const desc = document.getElementById('f-desc').value;
      const val = document.getElementById('f-value').value;
      const name = editing || title.toLowerCase().replace(/[^a-z0-9]+/g,'-').replace(/^-|-$/g,'');
      if (!name) return;
      const method = editing ? 'PUT' : 'POST';
      const url = editing ? API+'/'+name : API;
      try {
        const res = await fetch(url, { method, headers:{'Content-Type':'application/json'},
          body: JSON.stringify({metadata:{name}, spec:{title,description:desc,value:val}})});
        if (!res.ok) { const d = await res.json(); showError(d.message||'Save failed'); return; }
        cancelEdit(); load();
      } catch(e) { showError('Network error'); }
    };

    function editItem(name, title, desc, val) {
      editing = name;
      document.getElementById('f-title').value = title;
      document.getElementById('f-desc').value = desc;
      document.getElementById('f-value').value = val;
      document.getElementById('btn-save').textContent = 'Update';
      document.getElementById('btn-cancel').style.display = '';
    }

    function cancelEdit() {
      editing = null;
      document.getElementById('form').reset();
      document.getElementById('btn-save').textContent = 'Create';
      document.getElementById('btn-cancel').style.display = 'none';
    }

    async function del(name) {
      await fetch(API+'/'+name, {method:'DELETE'}); load();
    }

    function esc(s) { const d = document.createElement('div'); d.textContent = s; return d.innerHTML; }
    function showError(msg) { const e = document.getElementById('error'); e.textContent = msg; e.style.display = ''; }
    function hideError() { document.getElementById('error').style.display = 'none'; }

    load();
  </script>
</body>
</html>
`

const tmplAppTsx = `import React, { useEffect, useState } from 'react';

const API_BASE = ` + "`" + `/apis/{{.GroupName}}/v1alpha1/namespaces/default/{{.KindPlural}}` + "`" + `;

interface {{.KindName}} {
  metadata: { name: string; resourceVersion?: string };
  spec: { title: string; description?: string; value?: string };
}

export default function App() {
  const [items, setItems] = useState<{{.KindName}}[]>([]);
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [value, setValue] = useState('');
  const [editing, setEditing] = useState<string | null>(null);
  const [error, setError] = useState('');

  const load = async () => {
    try {
      const res = await fetch(API_BASE);
      const data = await res.json();
      setItems(data.items || []);
      setError('');
    } catch (e) {
      setError('Failed to load items');
    }
  };

  useEffect(() => { load(); }, []);

  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    const name = editing || title.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
    if (!name) return;

    const method = editing ? 'PUT' : 'POST';
    const url = editing ? API_BASE + '/' + name : API_BASE;
    const body = {
      metadata: { name },
      spec: { title, description, value },
    };

    try {
      const res = await fetch(url, {
        method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      if (!res.ok) {
        const err = await res.json();
        setError(err.message || 'Save failed');
        return;
      }
      setTitle('');
      setDescription('');
      setValue('');
      setEditing(null);
      load();
    } catch {
      setError('Network error');
    }
  };

  const remove = async (name: string) => {
    await fetch(API_BASE + '/' + name, { method: 'DELETE' });
    load();
  };

  const edit = (item: {{.KindName}}) => {
    setEditing(item.metadata.name);
    setTitle(item.spec.title);
    setDescription(item.spec.description || '');
    setValue(item.spec.value || '');
  };

  return (
    <div style={{ fontFamily: 'system-ui, sans-serif', maxWidth: 720, margin: '2rem auto', padding: '0 1rem' }}>
      <h1 style={{ borderBottom: '2px solid #f60', paddingBottom: 8 }}>{{.KindName}} Manager</h1>

      {error && <div style={{ color: '#c00', marginBottom: 16 }}>{error}</div>}

      <form onSubmit={save} style={{ display: 'flex', flexDirection: 'column', gap: 8, marginBottom: 24 }}>
        <input
          placeholder="Title (required)"
          value={title}
          onChange={e => setTitle(e.target.value)}
          required
          style={{ padding: 8, fontSize: 14 }}
        />
        <input
          placeholder="Description"
          value={description}
          onChange={e => setDescription(e.target.value)}
          style={{ padding: 8, fontSize: 14 }}
        />
        <input
          placeholder="Value"
          value={value}
          onChange={e => setValue(e.target.value)}
          style={{ padding: 8, fontSize: 14 }}
        />
        <div style={{ display: 'flex', gap: 8 }}>
          <button type="submit" style={{ padding: '8px 16px', background: '#f60', color: '#fff', border: 'none', cursor: 'pointer' }}>
            {editing ? 'Update' : 'Create'}
          </button>
          {editing && (
            <button type="button" onClick={() => { setEditing(null); setTitle(''); setDescription(''); setValue(''); }}
              style={{ padding: '8px 16px', cursor: 'pointer' }}>
              Cancel
            </button>
          )}
        </div>
      </form>

      <table style={{ width: '100%', borderCollapse: 'collapse' }}>
        <thead>
          <tr style={{ borderBottom: '1px solid #ddd', textAlign: 'left' }}>
            <th style={{ padding: 8 }}>Name</th>
            <th style={{ padding: 8 }}>Title</th>
            <th style={{ padding: 8 }}>Description</th>
            <th style={{ padding: 8 }}>Value</th>
            <th style={{ padding: 8 }}>Actions</th>
          </tr>
        </thead>
        <tbody>
          {items.map(item => (
            <tr key={item.metadata.name} style={{ borderBottom: '1px solid #eee' }}>
              <td style={{ padding: 8, fontFamily: 'monospace', fontSize: 13 }}>{item.metadata.name}</td>
              <td style={{ padding: 8 }}>{item.spec.title}</td>
              <td style={{ padding: 8 }}>{item.spec.description}</td>
              <td style={{ padding: 8 }}>{item.spec.value}</td>
              <td style={{ padding: 8 }}>
                <button onClick={() => edit(item)} style={{ marginRight: 4, cursor: 'pointer' }}>Edit</button>
                <button onClick={() => remove(item.metadata.name)} style={{ color: '#c00', cursor: 'pointer' }}>Delete</button>
              </td>
            </tr>
          ))}
          {items.length === 0 && (
            <tr><td colSpan={5} style={{ padding: 16, textAlign: 'center', color: '#999' }}>No items yet. Create one above!</td></tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
`

const tmplPluginJSON = `{
  "type": "app",
  "name": "{{.KindName}} Manager",
  "id": "{{.KindLower}}-manager",
  "info": {
    "description": "Manage {{.KindName}} resources via the Grafana app platform",
    "author": { "name": "Grafana App Platform" },
    "version": "0.1.0"
  }
}
`

const tmplTypesTS = `// Auto-generated TypeScript types for {{.KindName}}

export interface {{.KindName}}Spec {
  title: string;
  description?: string;
  value?: string;
}

export interface {{.KindName}} {
  apiVersion: string;
  kind: '{{.KindName}}';
  metadata: {
    name: string;
    namespace: string;
    resourceVersion?: string;
    labels?: Record<string, string>;
    creationTimestamp?: string;
  };
  spec: {{.KindName}}Spec;
  status?: Record<string, unknown>;
}

export interface {{.KindName}}List {
  apiVersion: string;
  kind: '{{.KindName}}List';
  items: {{.KindName}}[];
}
`

const tmplMakefile = `.PHONY: run build test clean

APP_NAME = {{.AppName}}
PORT ?= {{.Port}}

run:
	PORT=$(PORT) go run .

build:
	go build -o $(APP_NAME) .

test:
	go test ./...

clean:
	rm -f $(APP_NAME) {{.KindLower}}s.db
`

const tmplReadme = `# {{.AppName}}

A Grafana app-platform app built with the quickstart CLI.

## Quick Start

` + "```" + `bash
# Run the app (no K8s required)
make run

# Or directly:
go run .
` + "```" + `

The app starts at ` + "`" + `http://localhost:{{.Port}}` + "`" + ` with:
- **CRUD UI** at the root path
- **REST API** at ` + "`" + `/apis/{{.GroupName}}/v1alpha1/namespaces/default/{{.KindPlural}}` + "`" + `

## API Examples

` + "```" + `bash
# Create
curl -X POST http://localhost:{{.Port}}/apis/{{.GroupName}}/v1alpha1/namespaces/default/{{.KindPlural}} \
  -H "Content-Type: application/json" \
  -d '{"metadata":{"name":"example-1"},"spec":{"title":"My First {{.KindName}}","description":"Hello world"}}'

# List
curl http://localhost:{{.Port}}/apis/{{.GroupName}}/v1alpha1/namespaces/default/{{.KindPlural}}

# Get
curl http://localhost:{{.Port}}/apis/{{.GroupName}}/v1alpha1/namespaces/default/{{.KindPlural}}/example-1

# Delete
curl -X DELETE http://localhost:{{.Port}}/apis/{{.GroupName}}/v1alpha1/namespaces/default/{{.KindPlural}}/example-1
` + "```" + `

## Project Structure

` + "```" + `
{{.AppName}}/
  main.go              # Entrypoint — starts embedded server + frontend
  kinds/
    {{.KindLower}}.go      # Kind definition with Go struct tags
  plugin/
    src/
      App.tsx          # React CRUD UI
      plugin.json      # Plugin metadata
      types.ts         # Generated TypeScript types
  go.mod
  Makefile
  README.md
` + "```" + `

## Architecture

` + "```" + `
Browser → local.Server (K8s REST API) → SQLite
` + "```" + `

The embedded server implements K8s-compatible REST routes. When you're ready
for production, swap ` + "`" + `local.Server` + "`" + ` for a real K8s client — your API routes,
frontend code, and business logic remain unchanged.

## Kind Definition

Edit ` + "`" + `kinds/{{.KindLower}}.go` + "`" + ` to modify the {{.KindName}} schema. The struct tags
define validation, JSON field names, and descriptions:

` + "```" + `go
type {{.KindName}}Spec struct {
    Title       string ` + "`" + `json:"title" validate:"required" description:"Display title"` + "`" + `
    Description string ` + "`" + `json:"description,omitempty" description:"Optional description"` + "`" + `
    Value       string ` + "`" + `json:"value,omitempty" description:"Custom value"` + "`" + `
}
` + "```" + `
`
