#!/bin/bash
# ═══════════════════════════════════════════════════════════════
# CyberStrikeAI - Local Embedding Server (vLLM)
# Runs multilingual-e5-small for knowledge base embeddings
# All LLM inference is external (configured in config.yaml)
# ═══════════════════════════════════════════════════════════════
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

# ─── Config ───────────────────────────────────────────────────────
MODEL="intfloat/multilingual-e5-small"
MODEL_NAME="multilingual-e5-small"
PORT=8102
HOST="0.0.0.0"
MAX_MODEL_LEN=512
GPU_MEM=0.5          # 50% GPU memory (~6GB) - plenty for e5-small
DTYPE="half"
LOG_FILE="/tmp/vllm_embed.log"

log()  { echo -e "${GREEN}[+]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }
err()  { echo -e "${RED}[x]${NC} $1"; }
info() { echo -e "${CYAN}[*]${NC} $1"; }

echo "═══════════════════════════════════════════════════════════════"
echo "  CyberStrikeAI - Local Embedding Server"
echo "  Model: $MODEL_NAME ($MODEL)"
echo "  Port:  $PORT"
echo "═══════════════════════════════════════════════════════════════"
echo ""

# ─── Check: already running? ─────────────────────────────────────
if curl -s --connect-timeout 2 "http://127.0.0.1:$PORT/v1/models" >/dev/null 2>&1; then
    log "Embedding server already running on :$PORT"
    # Quick health check
    RESULT=$(curl -s "http://127.0.0.1:$PORT/v1/embeddings" \
        -H "Content-Type: application/json" \
        -d "{\"model\":\"$MODEL_NAME\",\"input\":\"health check\"}" 2>/dev/null)
    DIM=$(echo "$RESULT" | python3 -c "import sys,json; print(len(json.load(sys.stdin)['data'][0]['embedding']))" 2>/dev/null || echo "?")
    log "Health check OK - dimension: $DIM"
    exit 0
fi

# ─── Check: vLLM installed? ──────────────────────────────────────
info "Checking vLLM..."
if command -v vllm &>/dev/null; then
    log "vLLM found: $(which vllm)"
elif python3 -c "import vllm" 2>/dev/null; then
    log "vLLM importable (no CLI wrapper)"
else
    err "vLLM not installed"
    echo ""
    echo "  Install with:  pip install vllm"
    echo "  Requires:      NVIDIA GPU with CUDA support"
    echo ""
    exit 1
fi

# ─── Check: GPU ──────────────────────────────────────────────────
info "Checking GPU..."
if command -v nvidia-smi &>/dev/null; then
    GPU_INFO=$(nvidia-smi --query-gpu=name,memory.total,memory.free --format=csv,noheader,nounits 2>/dev/null | head -1)
    if [ -n "$GPU_INFO" ]; then
        GPU_NAME=$(echo "$GPU_INFO" | cut -d',' -f1 | xargs)
        GPU_TOTAL=$(echo "$GPU_INFO" | cut -d',' -f2 | xargs)
        GPU_FREE=$(echo "$GPU_INFO" | cut -d',' -f3 | xargs)
        log "GPU: $GPU_NAME - ${GPU_FREE}MB free / ${GPU_TOTAL}MB total"

        # Check if enough memory (e5-small needs ~500MB)
        if [ "$GPU_FREE" -lt 500 ]; then
            warn "Low GPU memory (${GPU_FREE}MB free). Model needs ~500MB."
            warn "Reducing GPU memory fraction..."
            GPU_MEM=0.3
        fi
    else
        err "nvidia-smi found but no GPU detected"
        exit 1
    fi
else
    err "nvidia-smi not found - CUDA GPU required for vLLM"
    exit 1
fi

# ─── Kill stale vLLM processes on same port ──────────────────────
if pgrep -f "vllm.*$PORT" >/dev/null 2>&1; then
    warn "Killing stale vLLM on :$PORT..."
    pkill -9 -f "vllm.*$PORT" 2>/dev/null
    sleep 2
fi

# ─── Launch ──────────────────────────────────────────────────────
info "Starting vLLM embedding server..."
info "Model: $MODEL"
info "Port:  $PORT"
info "Log:   $LOG_FILE"
echo ""

nohup vllm serve "$MODEL" \
    --host "$HOST" \
    --port "$PORT" \
    --served-model-name "$MODEL_NAME" \
    --runner pooling \
    --convert embed \
    --max-model-len "$MAX_MODEL_LEN" \
    --gpu-memory-utilization "$GPU_MEM" \
    --dtype "$DTYPE" \
    > "$LOG_FILE" 2>&1 &

VLLM_PID=$!
info "vLLM PID: $VLLM_PID"

# ─── Wait for startup ───────────────────────────────────────────
info "Waiting for server to start..."
MAX_WAIT=60
WAITED=0
while [ $WAITED -lt $MAX_WAIT ]; do
    if curl -s --connect-timeout 1 "http://127.0.0.1:$PORT/v1/models" >/dev/null 2>&1; then
        break
    fi
    # Check if process died
    if ! kill -0 "$VLLM_PID" 2>/dev/null; then
        err "vLLM process died during startup"
        echo "  Check log: tail -20 $LOG_FILE"
        tail -10 "$LOG_FILE"
        exit 1
    fi
    sleep 2
    WAITED=$((WAITED+2))
    echo -ne "\r  [${WAITED}s/${MAX_WAIT}s] waiting..."
done
echo ""

if [ $WAITED -ge $MAX_WAIT ]; then
    err "Timeout waiting for vLLM to start ($MAX_WAIT seconds)"
    echo "  Check log: tail -20 $LOG_FILE"
    tail -10 "$LOG_FILE"
    exit 1
fi

# ─── Health check ────────────────────────────────────────────────
info "Running health check..."
RESULT=$(curl -s "http://127.0.0.1:$PORT/v1/embeddings" \
    -H "Content-Type: application/json" \
    -d "{\"model\":\"$MODEL_NAME\",\"input\":\"cybersecurity penetration testing\"}")

DIM=$(echo "$RESULT" | python3 -c "import sys,json; print(len(json.load(sys.stdin)['data'][0]['embedding']))" 2>/dev/null)

if [ -n "$DIM" ] && [ "$DIM" -gt 0 ] 2>/dev/null; then
    echo ""
    echo "═══════════════════════════════════════════════════════════════"
    echo -e "  ${GREEN}Embedding server running${NC}"
    echo ""
    echo "  PID:        $VLLM_PID"
    echo "  Model:      $MODEL_NAME"
    echo "  Dimension:  $DIM"
    echo "  Endpoint:   http://127.0.0.1:$PORT/v1/embeddings"
    echo ""
    echo "  config.yaml should have:"
    echo "    knowledge:"
    echo "      enabled: true"
    echo "      embedding:"
    echo "        provider: openai"
    echo "        model: $MODEL_NAME"
    echo "        base_url: http://127.0.0.1:$PORT/v1"
    echo "        api_key: none"
    echo ""
    echo "  Stop:  kill $VLLM_PID"
    echo "  Logs:  tail -f $LOG_FILE"
    echo "═══════════════════════════════════════════════════════════════"
else
    err "Health check failed - embedding response invalid"
    echo "  Raw response: $RESULT"
    echo "  Check log: tail -20 $LOG_FILE"
    exit 1
fi
