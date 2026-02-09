#!/usr/bin/env bash
set -euo pipefail

# 切到项目根目录（脚本在 scripts/ 下）
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROTO_DIR="${ROOT_DIR}/shared/proto"
OUT_DIR="${ROOT_DIR}/shared/gen"

# 依赖检查
command -v protoc >/dev/null 2>&1 || { echo "ERROR: protoc not found in PATH"; exit 1; }
command -v protoc-gen-go >/dev/null 2>&1 || { echo "ERROR: protoc-gen-go not found in PATH (go install ...)"; exit 1; }
command -v protoc-gen-go-grpc >/dev/null 2>&1 || { echo "ERROR: protoc-gen-go-grpc not found in PATH (go install ...)"; exit 1; }

mkdir -p "${OUT_DIR}"

# 找到所有 proto 文件
mapfile -t PROTOS < <(find "${PROTO_DIR}" -name "*.proto" -type f | sort)

if [ ${#PROTOS[@]} -eq 0 ]; then
  echo "No .proto files found under ${PROTO_DIR}"
  exit 0
fi

echo "Generating pb.go for ${#PROTOS[@]} proto files..."
protoc \
  --proto_path="${PROTO_DIR}" \
  --go_out="${OUT_DIR}" --go_opt=paths=source_relative \
  --go-grpc_out="${OUT_DIR}" --go-grpc_opt=paths=source_relative \
  "${PROTOS[@]}"

echo "Done. Output: ${OUT_DIR}"
