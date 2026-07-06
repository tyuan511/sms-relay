#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

COMPOSE_FILE="$ROOT_DIR/docker-compose.prod.yml"
ENV_FILE="$ROOT_DIR/.env"
VERSION_FILE="$ROOT_DIR/.deploy-version"

DOCKER_USER="tangge"
SERVER_IMAGE="${DOCKER_USER}/sms-relay-server"
WEB_IMAGE="${DOCKER_USER}/sms-relay-web"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }
step()  { echo -e "${BLUE}[STEP]${NC} $*"; }

COMPOSE=()

usage() {
  local current=""
  if [[ -f "$VERSION_FILE" ]]; then
    current="$(tr -d '[:space:]' < "$VERSION_FILE")"
  fi

  cat <<EOF
SMS Relay 生产部署 — 从 Docker Hub 拉取指定版本

用法: ./deploy.sh <版本|命令>

部署:
  ./deploy.sh 0.1.0          部署指定版本
  ./deploy.sh latest         部署 latest 标签
  ./deploy.sh v0.1.0         支持 v 前缀

管理:
  stop                       停止服务
  restart                    重启当前已部署版本
  logs                       查看日志
  status                     查看运行状态
  pull <版本>                仅拉取镜像，不启动

$( [[ -n "$current" ]] && echo "当前已部署版本: $current" )

镜像:
  ${SERVER_IMAGE}:<version>
  ${WEB_IMAGE}:<version>

首次部署:
  1. 复制 .env.example 为 .env
  2. 设置 PUBLIC_URL=https://你的域名
  3. 配置宿主机 nginx 反代到 127.0.0.1:\${WEB_PORT}（参考 deploy/nginx.example.conf）
  4. ./deploy.sh <version>

架构说明:
  公网 nginx → Docker web 容器（单入口）
  web 容器内自动将 /api/ 转发给 server 容器
  无需单独暴露 API 端口

示例:
  ./deploy.sh 0.1.0
  ./deploy.sh stop
  ./deploy.sh logs
EOF
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    error "未找到命令: $1"
    exit 1
  fi
}

detect_compose() {
  if docker compose version >/dev/null 2>&1; then
    COMPOSE=(docker compose -f "$COMPOSE_FILE")
  elif command -v docker-compose >/dev/null 2>&1; then
    COMPOSE=(docker-compose -f "$COMPOSE_FILE")
  else
    error "需要 Docker Compose（docker compose 或 docker-compose）"
    exit 1
  fi
}

gen_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -base64 32 | tr -d '\n'
  else
    docker run --rm alpine:3.21 sh -c 'apk add -q openssl >/dev/null && openssl rand -base64 32' | tr -d '\n'
  fi
}

sed_inplace() {
  if [[ "$(uname)" == "Darwin" ]]; then
    sed -i '' "$@"
  else
    sed -i "$@"
  fi
}

ensure_env() {
  if [[ ! -f "$ENV_FILE" ]]; then
    if [[ -f "$ROOT_DIR/.env.example" ]]; then
      info "首次部署，从 .env.example 创建 .env"
      cp "$ROOT_DIR/.env.example" "$ENV_FILE"
    else
      error "缺少 .env，请创建 $ENV_FILE"
      exit 1
    fi
  fi

  # shellcheck disable=SC1091
  set -a; source "$ENV_FILE"; set +a

  local changed=false

  if [[ -z "${JWT_SECRET:-}" ]]; then
    JWT_SECRET="$(gen_secret)"
    sed_inplace "s|^JWT_SECRET=.*|JWT_SECRET=${JWT_SECRET}|" "$ENV_FILE"
    changed=true
    info "已自动生成 JWT_SECRET"
  fi

  if [[ -z "${PASSWORD_PEPPER:-}" ]]; then
    PASSWORD_PEPPER="$(gen_secret)"
    if grep -q '^PASSWORD_PEPPER=' "$ENV_FILE"; then
      sed_inplace "s|^PASSWORD_PEPPER=.*|PASSWORD_PEPPER=${PASSWORD_PEPPER}|" "$ENV_FILE"
    else
      echo "PASSWORD_PEPPER=${PASSWORD_PEPPER}" >> "$ENV_FILE"
    fi
    changed=true
    info "已自动生成 PASSWORD_PEPPER"
  fi

  if [[ -z "${DATABASE_ENCRYPTION_KEY:-}" ]]; then
    DATABASE_ENCRYPTION_KEY="$(gen_secret)"
    sed_inplace "s|^DATABASE_ENCRYPTION_KEY=.*|DATABASE_ENCRYPTION_KEY=${DATABASE_ENCRYPTION_KEY}|" "$ENV_FILE"
    changed=true
    info "已自动生成 DATABASE_ENCRYPTION_KEY"
  fi

  if [[ "$changed" == true ]]; then
    warn "密钥已写入 .env，请妥善备份（丢失后无法解密已有数据）"
  fi

  if [[ -z "${CORS_ORIGIN:-}" && -n "${PUBLIC_URL:-}" ]]; then
    CORS_ORIGIN="$PUBLIC_URL"
    if grep -q '^CORS_ORIGIN=' "$ENV_FILE"; then
      sed_inplace "s|^CORS_ORIGIN=.*|CORS_ORIGIN=${CORS_ORIGIN}|" "$ENV_FILE"
    else
      echo "CORS_ORIGIN=${CORS_ORIGIN}" >> "$ENV_FILE"
    fi
    info "已从 PUBLIC_URL 设置 CORS_ORIGIN=${CORS_ORIGIN}"
  fi

  if [[ -z "${CORS_ORIGIN:-}" ]]; then
    warn "请设置 PUBLIC_URL 或 CORS_ORIGIN（如 https://sms.example.com）"
  fi

  # shellcheck disable=SC1091
  set -a; source "$ENV_FILE"; set +a
}

normalize_version() {
  local version="$1"
  version="${version#v}"
  if [[ "$version" == "latest" ]]; then
    echo "latest"
    return
  fi
  if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    error "无效版本号: $1（需要 x.y.z、latest，或 vX.Y.Z）"
    exit 1
  fi
  echo "$version"
}

read_deployed_version() {
  if [[ -f "$VERSION_FILE" ]]; then
    tr -d '[:space:]' < "$VERSION_FILE"
  fi
}

save_deployed_version() {
  printf '%s\n' "$1" > "$VERSION_FILE"
}

wait_healthy() {
  local web_port="${WEB_PORT:-8080}"
  local url="http://127.0.0.1:${web_port}/api/v1/health"
  local max=60
  step "等待服务就绪: $url"

  for i in $(seq 1 "$max"); do
    if curl -sf "$url" >/dev/null 2>&1; then
      info "服务已就绪 (${i}s)"
      return 0
    fi
    sleep 2
  done

  error "服务启动超时，请检查: ./deploy.sh logs"
  exit 1
}

cmd_pull() {
  local version="$1"
  step "拉取 ${SERVER_IMAGE}:${version}"
  docker pull "${SERVER_IMAGE}:${version}"
  step "拉取 ${WEB_IMAGE}:${version}"
  docker pull "${WEB_IMAGE}:${version}"
  info "镜像拉取完成"
}

cmd_deploy() {
  local version
  version="$(normalize_version "$1")"

  require_cmd docker
  require_cmd curl
  detect_compose
  ensure_env

  step "部署版本 v${version}"
  cmd_pull "$version"

  export SMS_RELAY_VERSION="$version"
  "${COMPOSE[@]}" pull
  "${COMPOSE[@]}" up -d --remove-orphans

  wait_healthy
  save_deployed_version "$version"

  local web_port="${WEB_PORT:-8080}"
  local public_url="${PUBLIC_URL:-${CORS_ORIGIN:-}}"

  echo ""
  echo -e "${GREEN}════════════════════════════════════════${NC}"
  echo -e "${GREEN}  SMS Relay 部署成功 v${version}${NC}"
  echo -e "${GREEN}════════════════════════════════════════${NC}"
  echo ""
  if [[ -n "$public_url" ]]; then
    echo "  公网地址:    ${public_url}"
    echo "  API 地址:    ${public_url}/api/v1"
    echo ""
    echo "  Android 配置:"
    echo "    服务器地址 → ${public_url}"
  else
    echo "  本机入口:    http://127.0.0.1:${web_port}"
    echo "  API 地址:    http://127.0.0.1:${web_port}/api/v1"
    echo ""
    echo "  请在 .env 设置 PUBLIC_URL，并配置宿主机 nginx 反代"
    echo "  参考: deploy/nginx.example.conf"
  fi
  echo ""
  echo "  Nginx 反代目标: http://127.0.0.1:${web_port}"
  echo ""
  echo "  常用命令:"
  echo "    ./deploy.sh logs"
  echo "    ./deploy.sh stop"
  echo "    ./deploy.sh restart"
  echo ""
}

cmd_stop() {
  detect_compose
  step "停止服务"
  "${COMPOSE[@]}" down
  info "已停止"
}

cmd_restart() {
  local version
  version="$(read_deployed_version)"
  if [[ -z "$version" ]]; then
    error "未找到已部署版本，请执行: ./deploy.sh <version>"
    exit 1
  fi
  info "重启当前版本 v${version}"
  detect_compose
  ensure_env
  export SMS_RELAY_VERSION="$version"
  "${COMPOSE[@]}" restart
  wait_healthy
  info "已重启"
}

cmd_logs() {
  detect_compose
  "${COMPOSE[@]}" logs -f --tail=100
}

cmd_status() {
  detect_compose
  local version
  version="$(read_deployed_version)"
  echo ""
  if [[ -n "$version" ]]; then
    info "已部署版本: v${version}"
  else
    warn "尚未记录部署版本"
  fi
  echo ""
  "${COMPOSE[@]}" ps
}

main() {
  local action="${1:-}"

  if [[ -z "$action" || "$action" == "-h" || "$action" == "--help" || "$action" == "help" ]]; then
    usage
    exit 0
  fi

  case "$action" in
    stop|down)
      cmd_stop
      ;;
    restart)
      cmd_restart
      ;;
    logs)
      cmd_logs
      ;;
    status|ps)
      cmd_status
      ;;
    pull)
      if [[ -z "${2:-}" ]]; then
        error "请指定版本: ./deploy.sh pull <version>"
        exit 1
      fi
      require_cmd docker
      cmd_pull "$(normalize_version "$2")"
      ;;
    *)
      cmd_deploy "$action"
      ;;
  esac
}

main "$@"
