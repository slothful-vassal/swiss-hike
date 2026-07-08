# Swiss Hike Wanderer Plugin

WASM provider plugin for importing public hike suggestions from Schweizer Wanderwege.

The plugin lists `hikes` from `/de/map-filter`, fetches each detail page from
`/de/wandervorschlaege/uuid/{uuid}`, parses title, description, metrics, photos,
and the GPX download link, then returns a Wanderer `TrailImport`.

Current status:

- Public hike listing works with the Schweizer Wanderwege CSRF token and session cookie.
- List and detail imports can emit debug timing logs for the provider calls and parsing steps when the debug logging option is enabled.
- Detail pages follow the provider's HTML meta refresh from UUID URLs to canonical URLs.
- Full detail import uses a Hitobito/Rando Community session. The plugin starts the
  Schweizer Wanderwege OAuth flow, posts the Rando login form with `email` and `password`,
  follows redirects back through `/login_check`, and reuses the resulting site cookies for
  detail and GPX downloads.
- The full login and detail import flow has been manually tested against a real account with access to Schweizer Wanderwege hike proposals. The redirect flow remains provider-dependent and should be rechecked after upstream login changes.

```sh
make build
```

The built bundle is written to `dist/swiss-hike`.

Release archives use the same layout as wanderer's first-party plugin assets:

```sh
make package
```

To publish, run the `Publish Release` GitHub Action manually from `main` with a
version such as `0.1.0`. The workflow updates `plugin.json`, commits the version
bump, creates the matching `vX.Y.Z` tag, and publishes
`wanderer-plugin-swiss-hike.tar.gz` plus `SHA256SUMS`.
