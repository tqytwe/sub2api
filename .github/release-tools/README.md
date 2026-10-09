# Release matrix

The release workflow builds the frontend once, then runs each configured Go target on its own Linux runner. `CGO_ENABLED=0` permits cross-compilation. The target matrix is read from `.goreleaser.yaml`, including its exclusions; simple releases select Linux amd64 only.

Each build uses GoReleaser OSS in snapshot mode with the selected release version and one target. Archive naming, bundled files, Go flags and release templates remain in the existing GoReleaser configurations. Every archive is accompanied by its source commit, target, version and SHA256. The publishing job verifies the complete matrix before building images or publishing. It uses GoReleaser's `extra_files` support to publish existing archives and checksums, with builds disabled. No Pro license is needed.

Go caches are isolated by target and refreshed on each source commit, with fallback to the preceding target cache. Save uses the original restore key, even if a build hook changes `go.sum`. Matrix jobs upload uniquely named artifacts. The publishing job extracts only the regular Linux binary from each verified archive and restores its executable permission before constructing Docker contexts. QEMU remains limited to runtime-image instructions. DockerHub images are omitted when its credentials are absent; GHCR is always retained. Simple mode still publishes only the amd64 GHCR image and the simple release description.

All build jobs use the commit resolved by `prepare`. Helper scripts come from the reviewed `play/main` workflow revision and are passed as a run-local artifact. Publication requires a fork tag on the current `origin/play/main` commit, a matching VERSION already reviewed in source, and a source lock matching the runtime policy. Only dry runs may select other application refs. The workflow serializes release runs to prevent simultaneous updates to moving image tags.

## Validate without publication

After this workflow is reviewed and merged into `play/main`, select **`play/main` as the workflow ref**. Select the application source independently with the `tag` input; for a dry run it may be a review branch or full commit SHA:

```bash
: "${APPLICATION_REF:?set a review branch or full application commit SHA}"
gh workflow run release.yml --ref play/main \
  -f tag="$APPLICATION_REF" -f dry_run=true -f simple_release=false
```

A dry run builds all selected archives and both runtime images, verifies artifact provenance and produces the final checksum file. It exports images locally as OCI archives instead of pushing them. It skips registry logins, GitHub Release publication, DockerHub description updates and Telegram notifications. No run writes or pushes VERSION; version changes must go through a `play/main` PR. Test the simple path separately with `simple_release=true`.

The full release footer links to the fork source build guide at the exact release commit. It does not recommend original-distribution installers. Provenance JSON and checksums identify the build; they are not signatures or deployment approval, and in-place binary installation remains disabled.

Dry-run artifacts are available in the Actions run, including `release-dry-run-report`. Compare job start/end times, GoReleaser's build duration and cache restore results. Do not present an initial cold-cache run as a warmed-cache benchmark; publishing network time is not measured by dry runs.

Helper checks:

```bash
python -m pip install -r .github/release-tools/requirements-release.txt
python -m unittest discover -s .github/release-tools -p 'test_release_matrix.py'
bash -n .github/release-tools/release-images.sh
```
