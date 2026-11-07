#!/usr/bin/env bash
# Atalhos de build/deploy do app Android. Uso: ./dev.sh <comando>
#
# Comandos:
#   emulator   abre o emulador em background (AVD_NOME, padrão trombadario_test)
#   build      builda o APK debug
#   test       roda os testes unitários
#   install    instala o APK debug já buildado no aparelho/emulador conectado
#   start      abre o app no aparelho/emulador conectado
#   run        build + install + start (o mais usado)
#   logs       segue o logcat do processo do app (precisa estar rodando)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

export JAVA_HOME="${JAVA_HOME:-/opt/homebrew/opt/openjdk@17}"
export ANDROID_HOME="${ANDROID_HOME:-/opt/homebrew/share/android-commandlinetools}"

ADB="$ANDROID_HOME/platform-tools/adb"
EMULATOR="$ANDROID_HOME/emulator/emulator"
# Reusa o AVD do trombadario: o nome é só do dispositivo do emulador.
AVD_NOME="${AVD_NOME:-trombadario_test}"
PACOTE="com.tempodetela"
ACTIVITY="$PACOTE/.MainActivity"
APK="$SCRIPT_DIR/app/build/outputs/apk/debug/app-debug.apk"

buildar() { (cd "$SCRIPT_DIR" && ./gradlew assembleDebug); }
testar() { (cd "$SCRIPT_DIR" && ./gradlew testDebugUnitTest); }
instalar() { "$ADB" install -r "$APK"; }
iniciar() { "$ADB" shell am start -n "$ACTIVITY"; }

comando="${1:-run}"

case "$comando" in
    emulator)
        echo "Abrindo o emulador $AVD_NOME (demora uns 30 s a 1 min)..."
        "$EMULATOR" -avd "$AVD_NOME" >/dev/null 2>&1 &
        disown
        ;;
    build) buildar ;;
    test) testar ;;
    install) instalar ;;
    start) iniciar ;;
    run)
        buildar
        instalar
        iniciar
        ;;
    logs)
        pid=$("$ADB" shell pidof -s "$PACOTE")
        echo "Seguindo o logcat do PID $pid (Ctrl+C pra sair)..."
        "$ADB" logcat --pid="$pid"
        ;;
    *)
        echo "Uso: ./dev.sh [emulator|build|test|install|start|run|logs]"
        exit 1
        ;;
esac
