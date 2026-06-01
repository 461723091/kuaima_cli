#!/usr/bin/env bash
set -euo pipefail

print_only="false"
amount="0"
plan_id="0"
payment_method="custom1_wxpay"
qr_image="true"
qr_file=""
api_key=""
base_url=""
username=""
password=""
verbose_cli="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --print-url)
      print_only="true"
      shift
      ;;
    --amount)
      amount="${2:-0}"
      shift 2
      ;;
    --plan-id)
      plan_id="${2:-0}"
      shift 2
      ;;
    --payment-method)
      payment_method="${2:-}"
      shift 2
      ;;
    --qr-image)
      qr_image="${2:-true}"
      shift 2
      ;;
    --no-qr-image)
      qr_image="false"
      shift
      ;;
    --qr-file)
      qr_file="${2:-}"
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
    --username)
      username="${2:-}"
      shift 2
      ;;
    --password)
      password="${2:-}"
      shift 2
      ;;
    --verbose-cli)
      verbose_cli="true"
      shift
      ;;
    -h|--help)
      cat <<'EOF'
Usage: kuaima-recharge.sh [options]

Options:
  --print-url
  --amount N
  --plan-id N
  --payment-method NAME
  --qr-image true|false
  --no-qr-image
  --qr-file PATH
  --api-key KEY
  --base-url URL
  --username USER
  --password PASS
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

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
skill_root="$(cd "$script_dir/.." && pwd)"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$os" in
  darwin)
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
    ;;
  linux)
    case "$arch" in
      arm64|aarch64)
        exe="$skill_root/bin/kuaima_cli-linux-arm64"
        ;;
      x86_64|amd64)
        exe="$skill_root/bin/kuaima_cli-linux-amd64"
        ;;
      *)
        echo "Unsupported Linux architecture: $arch" >&2
        exit 2
        ;;
    esac
    ;;
  *)
    echo "Unsupported operating system: $os" >&2
    exit 2
    ;;
esac

if [[ ! -f "$exe" ]]; then
  echo "Missing bundled CLI executable: $exe. Rebuild the skill package." >&2
  exit 1
fi

args=(recharge)

if [[ "$print_only" == "true" ]]; then
  args+=(-print-url)
fi
if [[ "${amount//[[:space:]]/}" != "" && "$amount" != "0" ]]; then
  args+=(-amount "$amount")
fi
if [[ "${plan_id//[[:space:]]/}" != "" && "$plan_id" != "0" ]]; then
  args+=(-plan-id "$plan_id")
fi
if [[ -n "$payment_method" ]]; then
  args+=(-payment-method "$payment_method")
fi
if [[ "$qr_image" == "false" ]]; then
  args+=(-qr-image false)
fi
if [[ -n "$qr_file" ]]; then
  args+=(-qr-file "$qr_file")
fi
if [[ -n "$api_key" ]]; then
  args+=(-api-key "$api_key")
fi
if [[ -n "$base_url" ]]; then
  args+=(-base-url "$base_url")
fi
if [[ -n "$username" ]]; then
  args+=(-username "$username")
fi
if [[ -n "$password" ]]; then
  args+=(-password "$password")
fi
if [[ "$verbose_cli" == "true" ]]; then
  args+=(-v)
fi

chmod +x "$exe" 2>/dev/null || true
exec "$exe" "${args[@]}"
