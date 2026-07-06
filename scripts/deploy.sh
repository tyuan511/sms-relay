#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    error "未找到命令: $1"
    exit 1
  fi
}

detect_compose() {
  if docker compose version >/dev/null 2>&1; then
    COMPOSE=(docker compose)
  elif command -v docker-compose >/dev/null 2>&1; then
    COMPOSE=(docker-compose)
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

ensure_env() {
  if [[ -f .env ]]; then
    info "使用已有 .env 配置"
    # shellcheck disable=SC1091
    set -a; source .env; set +a
    if [[ -z "${DATABASE_ENCRYPTION_KEY:-}" ]]; then
      error ".env 中 DATABASE_ENCRYPTION_KEY 为空，请填写或删除 .env 后重新运行"
      exit 1
    fi
    if [[ -z "${PASSWORD_PEPPER:-}" ]]; then
      error ".env 中 PASSWORD_PEPPER 为空，请填写或删除 .env 后重新运行"
      exit 1
    fi
    return
  fi

  info "首次部署，生成 .env ..."
  cp .env.example .env

  ENC_KEY="$(gen_secret)"
  JWT="$(gen_secret)"
  PEPPER="$(gen_secret)"

  if [[ "$(uname)" == "Darwin" ]]; then
    sed -i '' "s|^DATABASE_ENCRYPTION_KEY=.*|DATABASE_ENCRYPTION_KEY=${ENC_KEY}|" .env
    sed -i '' "s|^JWT_SECRET=.*|JWT_SECRET=${JWT}|" .env
    sed -i '' "s|^PASSWORD_PEPPER=.*|PASSWORD_PEPPER=${PEPPER}|" .env
  else
    sed -i "s|^DATABASE_ENCRYPTION_KEY=.*|DATABASE_ENCRYPTION_KEY=${ENC_KEY}|" .env
    sed -i "s|^JWT_SECRET=.*|JWT_SECRET=${JWT}|" .env
    sed -i "s|^PASSWORD_PEPPER=.*|PASSWORD_PEPPER=${PEPPER}|" .env
  fi

  warn "密钥已写入 .env，请妥善备份此文件（丢失后无法解密已有数据）"
}

wait_healthy() {
  local url="http://localhost:${API_PORT:-8080}/api/v1/health"
  local max=60
  info "等待服务就绪: $url"

  for i in $(seq 1 "$max"); do
    if curl -sf "$url" >/dev/null 2>&1; then
      info "服务已就绪 (${i}s)"
      return 0
    fi
    sleep 2
  done

  error "服务启动超时，请检查日志: ${COMPOSE[*]} logs"
  exit 1
}

cmd_up() {
  require_cmd docker
  require_cmd curl
  detect_compose
  ensure_env

  # shellcheck disable=SC1091
  set -a; source .env; set +a

  info "构建并启动容器 ..."
  "${COMPOSE[@]}" up -d --build --remove-orphans

  wait_healthy

  WEB_PORT="${WEB_PORT:-5173}"
  API_PORT="${API_PORT:-8080}"

  echo ""
  echo -e "${GREEN}════════════════════════════════════════${NC}"
  echo -e "${GREEN}  SMS Relay 部署成功${NC}"
  echo -e "${GREEN}════════════════════════════════════════${NC}"
  echo ""
  echo "  Web 控制台:  http://localhost:${WEB_PORT}"
  echo "  API 地址:    http://localhost:${API_PORT}/api/v1"
  echo ""
  echo "  Android 配置:"
  echo "    服务器地址 → http://<本机IP>:${API_PORT}"
  echo "    主密码     → Web 注册后保存"
  echo ""
  echo "  常用命令:"
  echo "    查看日志:  ./scripts/deploy.sh logs"
  echo "    停止服务:  ./scripts/deploy.sh stop"
  echo "    重启服务:  ./scripts/deploy.sh restart"
  echo ""
}

cmd_stop() {
  detect_compose
  info "停止容器 ..."
  "${COMPOSE[@]}" down
  info "已停止"
}

cmd_restart() {
  detect_compose
  ensure_env
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
  "${COMPOSE[@]}" ps
}

usage() {
  cat <<EOF
SMS Relay Docker 一键部署

用法: ./scripts/deploy.sh [命令]

命令:
  up        构建并启动（默认）
  stop      停止并移除容器
  restart   重启服务
  logs      查看日志
  status    查看容器状态

示例:
  ./scripts/deploy.sh
  ./scripts/deploy.sh up
  ./scripts/deploy.sh stop
EOF
}

ACTION="${1:-up}"

case "$ACTION" in
  up|start)   cmd_up ;;
  stop|down)  cmd_stop ;;
  restart)    cmd_restart ;;
  logs)       cmd_logs ;;
  status|ps)  cmd_status ;;
  -h|--help|help) usage ;;
  *)
    error "未知命令: $ACTION"
    usage
    exit 1
    ;;
esac
