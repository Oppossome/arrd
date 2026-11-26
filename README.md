# arrd

arrd is a self-hosted media manager for movies and TV. It runs as one program: it finds releases on the indexers you configure, downloads them with a built-in torrent engine, and imports them into your library folders. A web UI handles setup and day-to-day use.

## Features

- **Movies and TV.** Add titles from TMDB, browse and discover new ones, and track which movies and episodes are still missing.
- **Searching.** Search manually, sync RSS feeds, and search periodically for anything still missing.
- **Built-in downloads.** The torrent engine is built in, so no separate download client is needed. Downloads can be confined to a WireGuard tunnel: a static config, or Private Internet Access with port forwarding.
- **Importing.** Finished downloads are renamed into your library folders by hardlink, copy or move, and keep seeding.
- **Media servers.** Plex and Jellyfin can be told to rescan their libraries after an import.
- **One binary.** The web UI is embedded. There's an optional tray icon on Linux and Windows desktops, and the UI can optionally be served on your LAN as `arrd.local`.

## Getting started

You need Go and Node.js 22.

```sh
make run
```

This builds the web UI and starts arrd on <http://127.0.0.1:8484>. On first start, arrd prints a one-time **setup token** in its log. Open the web UI and enter it to start onboarding, where you set an admin password, your library folders, TMDB access, a VPN (optional) and indexers.

There is no config file. Everything else is configured in the web UI. Only these startup options exist:

| Flag               | Environment variable   | Default                                         |
| ------------------ | ---------------------- | ----------------------------------------------- |
| `-data-dir`        | `ARRD_DATA_DIR`        | `$XDG_DATA_HOME/arrd`, or `~/.local/share/arrd` |
| `-listen`          | `ARRD_LISTEN`          | `127.0.0.1:8484`                                |
| `-definitions-dir` | `ARRD_DEFINITIONS_DIR` | `DATA-DIR/definitions`                          |
| `-log-level`       | `ARRD_LOG_LEVEL`       | `info`                                          |

For a headless or container build without the tray icon:

```sh
cd web && npm ci && npm run build && cd ..
go build -tags notray ./cmd/arrd
```

## Development

- `cmd/arrd`: the entry point. `internal/`: all application code. `web/`: the SvelteKit UI.
- The HTTP API is defined in `tools/openapi.yaml`. After changing it, or the SQL in `internal/store`, regenerate the code with `go generate ./tools` (Go) and `cd web && npm run gen:api` (TypeScript). Don't edit generated files by hand.
- Tests: `go test ./...` and `cd web && npm run check && npm test`.

## Built with AI assistance

Much of this project was written with the help of AI coding tools. A person directed and reviewed the changes, but expect bugs and rough edges, and read the code before relying on it for anything important.

## Disclaimer

arrd is provided as is, with no warranty. **You are solely responsible for how you configure and use it.** That includes:

- the indexers, trackers and content you choose to search and download, and following the copyright and other laws where you live;
- your VPN and network setup, including whether traffic actually goes through the tunnel;
- exposing the web UI beyond your own machine, and keeping your password, API key and other credentials safe;
- the folders arrd is given, including files it moves, renames or replaces during imports.

The authors are not liable for any damage, data loss, legal consequences or other harm arising from your configuration or use of this software.

## License

arrd is released under the [MIT License](LICENSE). Its dependencies keep their own licenses. Notably, the torrent engine (`github.com/anacrolix/torrent` and related modules) is MPL-2.0, and `github.com/Kcchouette/cardigann-go` is LGPL-3.0.
