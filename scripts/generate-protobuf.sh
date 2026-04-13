#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export PATH="/opt/homebrew/bin:/usr/local/bin:${PATH}:$(go env GOPATH)/bin"

cd "$ROOT"

INCLUDE=""
for dir in /opt/homebrew/include /usr/local/include; do
  if [ -f "$dir/google/protobuf/timestamp.proto" ]; then
    INCLUDE="$INCLUDE -I $dir"
    break
  fi
done
if [ -z "$INCLUDE" ]; then
  echo "Could not find google/protobuf/timestamp.proto (install protobuf compiler)." >&2
  exit 1
fi

protoc \
  --go_out=rpcgen --go_opt=module=github.com/shaminabd/ap2-contracts-go \
  --go-grpc_out=rpcgen --go-grpc_opt=module=github.com/shaminabd/ap2-contracts-go \
  -I proto $INCLUDE \
  proto/orderpayment/v1/order_payment.proto

echo "Generated into rpcgen/apiv1/"
