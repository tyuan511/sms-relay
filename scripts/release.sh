#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

VERSION_FILE="$ROOT_DIR/VERSION"
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

DRY_RUN=false
NO_PUSH=false
NO_BUMP=false
PLATFORM="linux/amd64"
BUILDX_BUILDER="sms-relay-builder"
GIT_TAG=false

usage() {
  cat <<EOF
SMS Relay 发布脚本 — 版本管理、构建 Docker 并推送到 Docker Hub

用法: ./scripts/release.sh [选项] [版本]

版本参数（可选，互斥）:
  patch             自动递增 patch（默认）
  minor             递增 minor，patch 归零
  major             递增 major，minor/patch 归零
  x.y.z             手动指定版本号

选项:
  --dry-run         仅预览，不写入版本、不构建、不推送
  --no-push         构建镜像但不推送
  --no-bump         不修改版本，使用当前 VERSION 重新构建
  --platform ARCH   指定构建平台（默认 linux/amd64）
  --git-tag         创建 git tag v<version>（不自动 commit）
  -h, --help        显示帮助

示例:
  ./scripts/release.sh                  # patch 递增后构建并推送
  ./scripts/release.sh minor            # 0.1.0 → 0.2.0
  ./scripts/release.sh 1.0.0            # 手动指定版本
  ./scripts/release.sh --no-push patch  # 本地构建验证
  ./scripts/release.sh --dry-run        # 预览发布计划

推送目标:
  docker.io/${SERVER_IMAGE}:<version>
  docker.io/${WEB_IMAGE}:<version>
  以及对应的 :latest 标签

前置条件:
  - 已安装 Docker 与 buildx，且已登录: docker login
  - 默认使用 buildx 构建 linux/amd64 并推送至 Docker Hub
EOF
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    error "未找到命令: $1"
    exit 1
  fi
}

read_version() {
  if [[ ! -f "$VERSION_FILE" ]]; then
    echo "0.1.0"
    return
  fi
  tr -d '[:space:]' < "$VERSION_FILE"
}

validate_version() {
  if [[ ! "$1" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    error "无效版本号: $1（需要 semver 格式 x.y.z）"
    exit 1
  fi
}

bump_version() {
  local current="$1"
  local part="$2"
  local major minor patch

  IFS='.' read -r major minor patch <<< "$current"
  major="${major:-0}"
  minor="${minor:-0}"
  patch="${patch:-0}"

  case "$part" in
    major) echo "$((major + 1)).0.0" ;;
    minor) echo "${major}.$((minor + 1)).0" ;;
    patch) echo "${major}.${minor}.$((patch + 1))" ;;
    *) error "未知递增类型: $part"; exit 1 ;;
  esac
}

write_version() {
  local version="$1"
  if [[ "$DRY_RUN" == true ]]; then
    info "[dry-run] 写入版本 $version → $VERSION_FILE"
    return
  fi
  printf '%s\n' "$version" > "$VERSION_FILE"
  info "版本已更新: $version"
}

check_docker_login() {
  if [[ "$DRY_RUN" == true ]]; then
    return
  fi

  if ! docker info >/dev/null 2>&1; then
    error "Docker 未运行或无法访问，请启动 Docker Desktop"
    exit 1
  fi

  if [[ "$NO_PUSH" == true ]]; then
    return
  fi

  # docker-credential-desktop 等会在未登录时让 push 失败；提前检查 hub 认证
  if ! docker system info 2>/dev/null | grep -q "Username:"; then
    if ! grep -q "https://index.docker.io/v1/" "$HOME/.docker/config.json" 2>/dev/null; then
      warn "未检测到 Docker Hub 登录信息"
      echo ""
      echo "请先登录 Docker Hub:"
      echo "  docker login"
      echo ""
      read -r -p "已登录并继续? [y/N] " confirm
      if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
        info "已取消"
        exit 0
      fi
    fi
  fi
}

ensure_buildx() {
  if [[ "$DRY_RUN" == true ]]; then
    return
  fi

  if ! docker buildx version >/dev/null 2>&1; then
    error "需要 docker buildx，请升级 Docker Desktop 或安装 buildx 插件"
    exit 1
  fi

  if ! docker buildx inspect "$BUILDX_BUILDER" >/dev/null 2>&1; then
    info "创建 buildx builder: $BUILDX_BUILDER"
    docker buildx create --name "$BUILDX_BUILDER" --driver docker-container --bootstrap
  fi
  docker buildx use "$BUILDX_BUILDER" >/dev/null
}

docker_build() {
  local context="$1"
  local image="$2"
  local version="$3"
  local dockerfile="${4:-}"

  local tags=(
    -t "${image}:${version}"
    -t "${image}:latest"
  )

  local output_args=()
  if [[ "$NO_PUSH" == true ]]; then
    output_args=(--load)
  else
    output_args=(--push)
  fi

  if [[ "$DRY_RUN" == true ]]; then
    local file_display=""
    [[ -n "$dockerfile" ]] && file_display="-f $dockerfile "
    local push_display="--push"
    [[ "$NO_PUSH" == true ]] && push_display="--load"
    info "[dry-run] docker buildx build ${file_display}--platform ${PLATFORM} ${push_display} ${tags[*]} $context"
    return
  fi

  local cmd=(docker buildx build --platform "$PLATFORM")
  [[ -n "$dockerfile" ]] && cmd+=(-f "$dockerfile")
  cmd+=("${output_args[@]}")
  cmd+=("${tags[@]}" "$context")
  "${cmd[@]}"
}

docker_push() {
  local image="$1"
  local version="$2"

  if [[ "$NO_PUSH" == true ]]; then
    warn "跳过推送: ${image}:${version}"
    return
  fi

  if [[ "$DRY_RUN" == true ]]; then
    info "[dry-run] 镜像已在 buildx build --push 阶段推送: ${image}:${version}"
    return
  fi

  info "已推送: ${image}:${version} 与 ${image}:latest"
}

create_git_tag() {
  local version="$1"
  local tag="v${version}"

  if [[ "$GIT_TAG" != true ]]; then
    return
  fi

  if ! git rev-parse --git-dir >/dev/null 2>&1; then
    warn "非 git 仓库，跳过 tag"
    return
  fi

  if [[ "$DRY_RUN" == true ]]; then
    info "[dry-run] git tag $tag"
    return
  fi

  if git rev-parse "$tag" >/dev/null 2>&1; then
    warn "git tag $tag 已存在，跳过"
    return
  fi

  git tag -a "$tag" -m "Release $version"
  info "已创建 git tag: $tag（未自动 push，可执行 git push origin $tag）"
}

confirm_release() {
  local version="$1"

  if [[ "$DRY_RUN" == true ]]; then
    return
  fi

  echo ""
  echo -e "${GREEN}════════════════════════════════════════${NC}"
  echo -e "${GREEN}  发布计划${NC}"
  echo -e "${GREEN}════════════════════════════════════════${NC}"
  echo ""
  echo "  版本:     $version"
  echo "  Server:   ${SERVER_IMAGE}:${version}"
  echo "  Web:      ${WEB_IMAGE}:${version}"
  [[ -n "$PLATFORM" ]] && echo "  平台:     $PLATFORM"
  [[ "$NO_PUSH" == true ]] && echo "  推送:     否（--no-push）"
  [[ "$GIT_TAG" == true ]] && echo "  Git tag:  v${version}"
  echo ""

  read -r -p "确认发布? [y/N] " confirm
  if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
    info "已取消"
    exit 0
  fi
}

parse_args() {
  local bump_type="patch"
  local manual_version=""

  while [[ $# -gt 0 ]]; do
    case "$1" in
      -h|--help|help)
        usage
        exit 0
        ;;
      --dry-run)
        DRY_RUN=true
        shift
        ;;
      --no-push)
        NO_PUSH=true
        shift
        ;;
      --no-bump)
        NO_BUMP=true
        shift
        ;;
      --git-tag)
        GIT_TAG=true
        shift
        ;;
      --platform)
        PLATFORM="${2:?--platform 需要参数}"
        shift 2
        ;;
      patch|minor|major)
        bump_type="$1"
        shift
        ;;
      *)
        if [[ "$1" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
          manual_version="$1"
          shift
        else
          error "未知参数: $1"
          usage
          exit 1
        fi
        ;;
    esac
  done

  CURRENT_VERSION="$(read_version)"
  validate_version "$CURRENT_VERSION"

  if [[ "$NO_BUMP" == true ]]; then
    NEW_VERSION="$CURRENT_VERSION"
  elif [[ -n "$manual_version" ]]; then
    validate_version "$manual_version"
    NEW_VERSION="$manual_version"
  else
    NEW_VERSION="$(bump_version "$CURRENT_VERSION" "$bump_type")"
  fi

  validate_version "$NEW_VERSION"
}

main() {
  parse_args "$@"

  require_cmd docker

  info "当前版本: $CURRENT_VERSION"
  if [[ "$NEW_VERSION" != "$CURRENT_VERSION" ]]; then
    info "目标版本: $NEW_VERSION"
  else
    info "保持版本: $NEW_VERSION"
  fi

  check_docker_login
  ensure_buildx
  if [[ "$DRY_RUN" != true ]]; then
    confirm_release "$NEW_VERSION"
  else
    info "[dry-run] 跳过确认"
  fi

  step "构建 Server 镜像"
  docker_build "$ROOT_DIR/server" "$SERVER_IMAGE" "$NEW_VERSION"

  step "构建 Web 镜像"
  docker_build "$ROOT_DIR/web" "$WEB_IMAGE" "$NEW_VERSION"

  step "推送到 Docker Hub"
  docker_push "$SERVER_IMAGE" "$NEW_VERSION"
  docker_push "$WEB_IMAGE" "$NEW_VERSION"

  if [[ "$NEW_VERSION" != "$CURRENT_VERSION" ]]; then
    step "更新版本号"
    write_version "$NEW_VERSION"
  fi

  create_git_tag "$NEW_VERSION"

  echo ""
  echo -e "${GREEN}════════════════════════════════════════${NC}"
  echo -e "${GREEN}  发布完成 v${NEW_VERSION}${NC}"
  echo -e "${GREEN}════════════════════════════════════════${NC}"
  echo ""
  echo "  docker pull ${SERVER_IMAGE}:${NEW_VERSION}"
  echo "  docker pull ${WEB_IMAGE}:${NEW_VERSION}"
  echo ""
  if [[ "$NO_PUSH" != true && "$DRY_RUN" != true ]]; then
    echo "  Docker Hub:"
    echo "    https://hub.docker.com/r/${DOCKER_USER}/sms-relay-server"
    echo "    https://hub.docker.com/r/${DOCKER_USER}/sms-relay-web"
    echo ""
  fi
}

main "$@"
