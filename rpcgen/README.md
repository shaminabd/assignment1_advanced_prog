# Generated Go module (Assignment 2 — Repository B)

Module: `github.com/shaminabd/ap2-contracts-go`

This folder is the **generated** gRPC + protobuf Go API. In production you would:

1. Push **only** this content (plus `go.mod`, workflows) to a second GitHub repository.
2. Tag releases (e.g. `v1.0.0`) and depend on them from services:  
   `go get github.com/shaminabd/ap2-contracts-go@v1.0.0`
3. Run [`.github/workflows/generate-from-proto-repo.yml`](.github/workflows/generate-from-proto-repo.yml) after pushing new `.proto` files to Repository A.

In this monorepo, services use a `replace` directive to `../rpcgen` for local development.

Regenerate after editing protos:

```bash
./scripts/generate-protobuf.sh
```
