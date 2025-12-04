# Repository Guidelines

## Project Structure & Module Organization
- Root `Dockerfile` builds a Helm/Kubectl/Helmfile + ttyd tool image; container user `clovers` and workdir `/home/clovers`.
- `scripts/.bashrc` defines kubernetes aliases (`k`, `h`, `hf`), shell completions, and editor defaults; `scripts/.vimrc` enforces YAML-friendly 2-space indentation.
- `readme.md` documents the base build and run commands. Add helper scripts under `scripts/` and keep the image lean (prefer Alpine packages already present).

## Build, Test, and Development Commands
- `docker build . -t fgtech.fr/ttyd:latest -f docker/Dockerfile .` — builds the image from the repository root.
- `docker run --rm -it fgtech.fr/ttyd:latest /bin/bash` — opens an interactive shell to validate installed tools and aliases.
- `docker run --rm -p 7681:7681 fgtech.fr/ttyd:latest` — smoke-test the default `ttyd` entrypoint on port 7681.

## Coding Style & Naming Conventions
- Shell scripts: use Bash (Dockerfile sets `SHELL ["/bin/bash", "-c"]`), keep functions small, and prefer explicit `set -euo pipefail` in new scripts.
- YAML and Helmfiles: 2-space indent, spaces instead of tabs (matches `.vimrc`), descriptive key names, and environment-agnostic defaults.
- File naming: lower-kebab for scripts (`scripts/sync-config.sh`), lowercase resource names in manifests, and tag images with semver or git SHA when publishing.

## Testing Guidelines
- No automated suite is present; perform manual checks: successful `docker build`, container starts without errors, and `kubectl/helm/helmfile` report versions inside the container.
- Add ad-hoc verification scripts under `scripts/` when expanding tooling; name them `verify-*.sh` and document usage in `readme.md`.

## Commit & Pull Request Guidelines
- Use concise, imperative commits; prefer Conventional Commit prefixes (`feat:`, `chore:`, `fix:`) for clarity.
- Pull requests should describe the change, note testing performed (e.g., build/run commands), and link any tracking issue. For tooling changes, mention impacted images/tags and configuration expectations (env vars, kubeconfig mounting).

## Security & Configuration Tips
- Do not bake secrets or kubeconfig into the image. Mount credentials at runtime and rely on environment variables or mounted files.
- Keep packages minimal; remove unused tooling to reduce surface area and rebuild frequently to pick up Alpine security updates.
