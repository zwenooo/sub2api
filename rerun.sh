#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$script_dir"

temp_files=()
cleanup() {
  if ((${#temp_files[@]})); then
    rm -f "${temp_files[@]}" || true
  fi
}
trap cleanup EXIT

extract_compose_sub2api_image() {
  awk '
    function indent_len(line,   prefix) {
      prefix=line
      sub(/[^ \t].*$/, "", prefix)
      return length(prefix)
    }
    function trim(line) {
      sub(/^[ \t]+/, "", line)
      sub(/[ \t\r]+$/, "", line)
      return line
    }
    BEGIN {
      in_services=0
      services_indent=-1
      service_indent=-1
      in_sub2api=0
    }
    /^[ \t]*services:[ \t]*$/ {
      in_services=1
      services_indent=indent_len($0)
      next
    }
    in_services {
      if ($0 !~ /^[ \t]*$/ && indent_len($0) <= services_indent) {
        in_services=0
        in_sub2api=0
        next
      }

      if (match($0, /^[ \t]*[A-Za-z0-9_.-]+:[ \t]*$/)) {
        current_indent=indent_len($0)
        if (service_indent < 0 && current_indent > services_indent) {
          service_indent=current_indent
        }
        if (current_indent == service_indent) {
          name=$0
          sub(/^[ \t]*/, "", name)
          sub(/:.*/, "", name)
          in_sub2api=(name == "sub2api")
        }
      }

      if (in_sub2api && match($0, /^[ \t]*image:[ \t]*/)) {
        image=$0
        sub(/^[ \t]*image:[ \t]*/, "", image)
        print trim(image)
        exit
      }
    }
  ' "$compose_file"
}

resolve_base_image() {
  if [[ -n "${SUB2API_IMAGE:-}" ]]; then
    echo "$SUB2API_IMAGE"
    return 0
  fi

  local compose_image
  compose_image="$(extract_compose_sub2api_image)"
  compose_image="${compose_image%\"}"
  compose_image="${compose_image#\"}"
  compose_image="${compose_image%\'}"
  compose_image="${compose_image#\'}"

  if [[ -z "$compose_image" ]]; then
    return 1
  fi

  if [[ "$compose_image" =~ ^\$\{SUB2API_IMAGE:-([^}]*)\}$ ]]; then
    echo "${BASH_REMATCH[1]}"
    return 0
  fi

  if [[ "$compose_image" =~ ^\$\{SUB2API_IMAGE\}$ ]]; then
    return 1
  fi

  echo "$compose_image"
  return 0
}

print_help() {
  cat <<'EOF'
Usage:
  ./rerun.sh                  # pull sub2api image and recreate the stack
  ./rerun.sh --start          # start the stack without pulling images
  ./rerun.sh --restart        # restart existing containers without pulling images
  ./rerun.sh v1.2.3           # override SUB2API_IMAGE tag, then pull and recreate
  ./rerun.sh ghcr.io/x/y:tag  # override SUB2API_IMAGE fully, then pull and recreate

Env:
  COMPOSE_FILE=<path>         # compose file to use (default: docker-compose.actions.yml)
  SUB2API_IMAGE=<image:tag>   # image used by the compose file
EOF
}

default_compose_file="docker-compose.actions.yml"
compose_file="${COMPOSE_FILE:-$default_compose_file}"

if [[ ! -f "$compose_file" ]]; then
  echo "Compose file not found: $compose_file" >&2
  exit 1
fi

if [[ ! -f "./data/.env" ]]; then
  echo "Missing ./data/.env. Copy .env.actions.example to ./data/.env first." >&2
  exit 1
fi

if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
  compose_cmd=(docker compose)
elif command -v docker-compose >/dev/null 2>&1; then
  compose_cmd=(docker-compose)
else
  echo "Docker Compose not found. Install docker compose or docker-compose first." >&2
  exit 1
fi

mode="pull-restart"
override=""

while (($#)); do
  case "$1" in
    -h|--help)
      print_help
      exit 0
      ;;
    --start)
      mode="start"
      ;;
    --restart)
      mode="restart"
      ;;
    *)
      if [[ -n "$override" ]]; then
        echo "Only one image override is supported. Unexpected argument: $1" >&2
        exit 1
      fi
      override="$1"
      ;;
  esac
  shift
done

if [[ -n "$override" ]]; then
  if [[ "$override" == *"/"* || "$override" == *":"* || "$override" == *"@"* ]]; then
    export SUB2API_IMAGE="$override"
  else
    base_image="$(resolve_base_image || true)"
    if [[ -z "$base_image" ]]; then
      echo "Cannot derive base image; set SUB2API_IMAGE or pass a full image reference." >&2
      exit 1
    fi

    base_image="${base_image%@*}"
    if [[ "${base_image##*/}" == *":"* ]]; then
      base_image="${base_image%:*}"
    fi

    export SUB2API_IMAGE="${base_image}:${override}"
  fi
fi

compose_env_file_args=(--env-file ./data/.env)
compose_file_args=(-f "$compose_file")

override_compose_file=""
if [[ -n "${SUB2API_IMAGE:-}" ]]; then
  override_compose_file="$(mktemp -t sub2api.image.override.XXXXXX.yml)"
  temp_files+=("$override_compose_file")
  cat >"$override_compose_file" <<EOF
services:
  sub2api:
    image: "${SUB2API_IMAGE}"
EOF
  compose_file_args+=(-f "$override_compose_file")
fi

echo "Using compose file: $compose_file" >&2
if [[ -n "${SUB2API_IMAGE:-}" ]]; then
  echo "Using sub2api image: ${SUB2API_IMAGE}" >&2
fi

case "$mode" in
  pull-restart)
    "${compose_cmd[@]}" "${compose_env_file_args[@]}" "${compose_file_args[@]}" pull sub2api
    "${compose_cmd[@]}" "${compose_env_file_args[@]}" "${compose_file_args[@]}" up -d --remove-orphans --force-recreate --no-build
    ;;
  start)
    "${compose_cmd[@]}" "${compose_env_file_args[@]}" "${compose_file_args[@]}" up -d --remove-orphans --no-build
    ;;
  restart)
    "${compose_cmd[@]}" "${compose_env_file_args[@]}" "${compose_file_args[@]}" restart
    ;;
  *)
    echo "Unsupported mode: $mode" >&2
    exit 1
    ;;
esac

"${compose_cmd[@]}" "${compose_env_file_args[@]}" "${compose_file_args[@]}" ps
