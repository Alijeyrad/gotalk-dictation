# Flatpak Packaging

## Prerequisites

```bash
flatpak install flathub org.freedesktop.Platform//25.08 org.freedesktop.Sdk//25.08
flatpak install flathub org.freedesktop.Sdk.Extension.golang//25.08
```

## Vendoring dependencies (required for offline Flatpak build)

Flatpak builds run in a sandbox without network access. You must vendor the Go
module dependencies and include them in your release archive:

```bash
go mod vendor
# Commit vendor/ to git or include it in the release tarball
```

## Building locally

```bash
flatpak-builder --user --install --force-clean build-dir \
  packaging/flatpak/com.alijeyrad.GoTalkDictation.yml
```

## Running

```bash
flatpak run com.alijeyrad.GoTalkDictation
```

## Flathub submission

Before submitting to Flathub:
1. Update the `tag` and `commit` in the manifest to your release tag
2. Run `go mod vendor` and include `vendor/` in the release archive
3. Validate with `flatpak-builder --lint`
4. Follow the [Flathub submission guide](https://docs.flathub.org/docs/for-app-authors/submission/)
