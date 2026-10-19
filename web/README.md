# DART Filing Frontend (React)

A React-based Single Page Application (SPA) for viewing financial reports from the DART Filing backend.
Migrated from vanilla TypeScript to React + Vite.

## Features

- **Dashboard**: View reports for a specific company.
- **Reports List**: Browse reports with company and date filters sent to the API.
- **Expandable Rows**: View report details inline in the list view.
- **Raw Report Viewer**: View HTML, XML, or Text content of raw reports.

## Development

### Prerequisites

- Node.js **24.22.0 LTS** (repository `.nvmrc` / `.node-version`).
- npm **12.2.0** (declared in `web/package.json`).

The frontend pins React 19.3.1, Vite 8.3.4, `@vitejs/plugin-react` 6.1.2,
TypeScript 6.0.3, ESLint 10.13.0, and `typescript-eslint` 8.71.2.
TypeScript 6.0.3 is the latest compatible compiler: the parser's supported
TypeScript range is `>=4.8.4 <6.1.0`. Do not force TypeScript 7 through a peer
dependency override. See the [parser package metadata](https://registry.npmjs.org/typescript-eslint/8.71.2).

Run the commands below from the repository root with these Node/npm versions selected.

### Setup

1. Install dependencies:

```bash
npm --prefix web ci
```

2. Start Development Server (with HMR):

```bash
npm --prefix web run dev
```

Then open `http://localhost:5173` in your browser.

## Production Build

1. Build the project:

```bash
npm --prefix web run build
```

The output will be in the `dist/` directory.
The build keeps the previous browser output targets explicitly while using Vite 8.

2. Preview the production build locally:

```bash
npm --prefix web run preview
```

## Validation

```bash
npm --prefix web ls --depth=0
npm --prefix web run build
npm --prefix web run lint
npm --prefix web audit
```

Use `npm ci` to verify the checked-in manifest and lockfile together; do not edit
the lockfile's root dependency versions independently.

## API Configuration

In development, Vite proxies `/api` requests to `http://localhost:8080` (configurable via `VITE_API_PROXY_TARGET`).
For production builds, set `VITE_API_BASE_URL` to your API server origin.
