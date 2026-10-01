#!/usr/bin/env bash
# Runs the small-model benchmarks of llm/docs/EFFICIENT_SLM.md against one model in llama-server.
#   ./run.sh decide <model.gguf> <name> decider|chat   # routing, injection, yes/no facts
#   ./run.sh write  <model.gguf> <name> [llama-server flags]   # Spanish answers from tool data
set -euo pipefail
cd "$(dirname "$0")"
kind=$1 model=$2 name=$3; shift 3
style=chat
if [ "$kind" = decide ]; then style=${1:-chat}; shift || true; fi
llama-server -m "$model" --port 8080 --jinja -c 4096 -np 1 "$@" >/tmp/slm-bench-server.log 2>&1 &
pid=$!
trap 'kill $pid 2>/dev/null' EXIT
until curl -sf http://127.0.0.1:8080/health >/dev/null; do sleep 1; done
case $kind in
  decide) python3 decide_bench.py "$name" "$style" ;;
  write)  python3 write_bench.py "$name" ;;
esac
