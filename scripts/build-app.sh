#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ANDROID_DIR="$ROOT_DIR/android"

if [[ -z "${JAVA_HOME:-}" ]]; then
  for candidate in \
    "/opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home" \
    "/Applications/Android Studio.app/Contents/jbr/Contents/Home"; do
    if [[ -x "$candidate/bin/java" ]]; then
      export JAVA_HOME="$candidate"
      break
    fi
  done
fi

export ANDROID_HOME="${ANDROID_HOME:-$HOME/Library/Android/sdk}"

if [[ ! -x "$ANDROID_DIR/gradlew" ]]; then
  echo "未找到 android/gradlew，请先在 android 目录生成 Gradle Wrapper" >&2
  exit 1
fi

cd "$ANDROID_DIR"
./gradlew assembleRelease "$@"

APK="$ANDROID_DIR/app/build/outputs/apk/release/app-release.apk"
echo ""
echo "APK: $APK"
