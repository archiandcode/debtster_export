# export-debtster — file storage changes

This project previously used S3/MinIO for storing generated export files. That functionality has been removed — exports are now stored locally and served from the application.

Key configuration and behavior
- EXPORT_DIR (env) — directory where exported files are written (default: `./exports`).
- EXPORT_PUBLIC_PREFIX (env) — HTTP path prefix used to serve files (default: `/files`).
- EXTERNAL_URL (env) — optional absolute URL (e.g. `https://example.com:8060`) used for constructing `file_url` returned by the API. If unset, `file_url` is a relative path like `/files/<file>`.

How files are exposed
- Files are saved under `EXPORT_DIR` with a unique prefix (random hex + underscore) to avoid collisions, e.g. `d94b8b43a916d58b_debts_20251125_140206.xlsx`.
- The API `file_url` will contain the public path (either relative `/files/<name>` or absolute `https://host:port/files/<name>` when `EXTERNAL_URL` is set).
- The app exposes GET /files/{file} which returns the file and sets `Content-Disposition: attachment; filename="<original-name>"` so browsers download with the original filename.

Background cleanup
- The app runs a background goroutine that removes saved export files older than 30 minutes.

When upgrading
- Remove S3 credentials from environment and configure `EXPORT_DIR` and `EXPORT_PUBLIC_PREFIX` instead. Optionally set `EXTERNAL_URL` if you want API to return absolute file links.
