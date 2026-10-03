# Platrium Core Engine

This is the primary backend engine for Platrium, written in Go.

## Building and Serving

We use Nx to manage builds and executions to ensure consistency across the workspace. The Go backend supports several configurations to toggle Enterprise (EE) features and UI embedding via Go Build Tags (`//go:build ...`).

### Local Development (Default)

During normal local development, the Go server expects the Vite development server to be running (usually on `http://localhost:5173`). It uses an internal Reverse Proxy to forward UI requests (like `/login`), meaning you get instant Hot Module Reloading (HMR) natively without rebuilding the Go binary.

```bash
nx serve engine
```

**Environment Variables:**
- `DEV_UI_PROXY`: The URL of the frontend Vite dev server to proxy UI requests to. Defaults to `http://localhost:5173`.
- `PORT`: The port for the Go server. Defaults to `3000`.

### Enterprise Edition (EE) Development

To run the backend with Enterprise Edition features (e.g., SAML, advanced multi-tenancy) enabled, use the `dev-ee` configuration. This compiles the Go code with the `ee` build tag.

```bash
nx serve engine -c dev-ee
```

### Production Build (Embedded UI)

In production, the Go binary embeds the built static assets from the frontend `web` project to create a single standalone executable.

```bash
nx build engine -c release
```

Under the hood, this configuration will:
1. Clear any old UI builds.
2. Copy the newly compiled assets from `../web/dist` to `ui/dist`.
3. Compile the Go binary with the `embed_ui` build tag, locking the frontend statically inside the executable.

To build the Production binary with EE features enabled:

```bash
nx build engine -c release-ee
```
