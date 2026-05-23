#!/usr/bin/env bash
set -euo pipefail

prompt=""
output_dir="./outputs"
files=()
mask=""
size="auto"
quality="auto"
count="1"
format=""
compression="-1"
background="auto"
moderation=""
api_key=""
base_url=""
verbose_cli="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --prompt)
      prompt="${2:-}"
      shift 2
      ;;
    --output-dir)
      output_dir="${2:-}"
      shift 2
      ;;
    --file)
      files+=("${2:-}")
      shift 2
      ;;
    --mask)
      mask="${2:-}"
      shift 2
      ;;
    --size)
      size="${2:-}"
      shift 2
      ;;
    --quality)
      quality="${2:-}"
      shift 2
      ;;
    --count)
      count="${2:-}"
      shift 2
      ;;
    --format)
      format="${2:-}"
      shift 2
      ;;
    --compression)
      compression="${2:-}"
      shift 2
      ;;
    --background)
      background="${2:-}"
      shift 2
      ;;
    --moderation)
      moderation="${2:-}"
      shift 2
      ;;
    --api-key)
      api_key="${2:-}"
      shift 2
      ;;
    --base-url)
      base_url="${2:-}"
      shift 2
      ;;
    --verbose-cli)
      verbose_cli="true"
      shift
      ;;
    -h|--help)
      cat <<'EOF'
Usage: kuaima-image.sh --prompt TEXT [options]

Options:
  --output-dir DIR
  --file PATH_OR_URL       Repeat for multiple reference images.
  --mask PATH_OR_URL
  --size SIZE
  --quality auto|low|medium|high
  --count N
  --format png|jpeg|webp
  --compression N
  --background auto|transparent|opaque
  --moderation auto|low
  --api-key KEY
  --base-url URL
  --verbose-cli
EOF
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      exit 2
      ;;
  esac
done

if [[ -z "${prompt//[[:space:]]/}" ]]; then
  echo "Missing required --prompt." >&2
  exit 2
fi

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
skill_root="$(cd "$script_dir/.." && pwd)"

arch="$(uname -m)"
case "$arch" in
  arm64|aarch64)
    exe="$skill_root/bin/kuaima_cli-darwin-arm64"
    ;;
  x86_64|amd64)
    exe="$skill_root/bin/kuaima_cli-darwin-amd64"
    ;;
  *)
    echo "Unsupported macOS architecture: $arch" >&2
    exit 2
    ;;
esac

if [[ ! -f "$exe" ]]; then
  echo "Missing bundled CLI executable: $exe. Rebuild the skill package." >&2
  exit 1
fi

args=(
  image
  -image-model gpt-image-2
  -save-images "$output_dir"
  -image-size "$size"
  -image-quality "$quality"
  -image-count "$count"
  -image-background "$background"
)

if [[ -n "$format" ]]; then
  args+=(-image-output-format "$format")
fi
if [[ "$compression" != "-1" ]]; then
  args+=(-image-output-compression "$compression")
fi
if [[ -n "$moderation" ]]; then
  args+=(-image-moderation "$moderation")
fi
if [[ -n "$mask" ]]; then
  args+=(-image-mask "$mask")
fi
if [[ -n "$api_key" ]]; then
  args+=(-api-key "$api_key")
fi
if [[ -n "$base_url" ]]; then
  args+=(-base-url "$base_url")
fi
if [[ "$verbose_cli" == "true" ]]; then
  args+=(-v)
fi
for item in "${files[@]}"; do
  if [[ -n "$item" ]]; then
    args+=(-file "$item")
  fi
done

args+=("$prompt")

config_base="$output_dir"
if [[ -z "${config_base//[[:space:]]/}" ]]; then
  config_base="${TMPDIR:-/tmp}"
fi
config_dir="$config_base/.kuaima-config"
mkdir -p "$config_dir"

export KUAIMA_CONFIG_DIR="$config_dir"
chmod +x "$exe" 2>/dev/null || true
exec "$exe" "${args[@]}"
