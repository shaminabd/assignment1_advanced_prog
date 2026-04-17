# Generated Go module (Assignment 2 — Repository B)

Module: `github.com/shaminabd/ap2-contracts-go`

This folder is the **generated** gRPC + protobuf Go API. In production you would:

1. Push **only** this content (plus `go.mod`, workflows) to a second GitHub repository.
2. Tag releases (e.g. `v1.0.0`) and depend on them from services:  
   `go get github.com/shaminabd/ap2-contracts-go@v1.0.0`
3. Run [`.github/workflows/generate.yml`](.github/workflows/generate.yml) after pushing new `.proto` files to Repository A.

In this monorepo, services use a `replace` directive to `../rpcgen` for local development.

Regenerate after editing protos:

```bash
./scripts/generate-protobuf.sh
```

## GitHub Actions setup (Repository B)

1. Push this folder to your generated-code repo.
2. In repository settings, create secret `PAT_TOKEN`.
3. Open `.github/workflows/generate.yml` and replace:
   - `PROTO_REPO` with your Repository A slug (`your_user/your_proto_repo`)
   - `PROTO_REF` with your proto branch (`main` or `master`)
   - `GO_MODULE` with your generated repo module path
4. Open the **Actions** tab and run workflow **Generate Go from Proto Repository**.
5. Create a release tag manually (`v1.0.0`) in GitHub Releases.
