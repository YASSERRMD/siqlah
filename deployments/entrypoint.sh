#!/bin/sh
set -e

MODE="${1:-server}"
KEY_FILE="/data/operator.key"

generate_key_if_missing() {
    if [ ! -f "$KEY_FILE" ]; then
        echo "Generating operator keypair..."
        /usr/local/bin/witness keygen --out "$KEY_FILE"
        echo "Operator key written to $KEY_FILE"
    fi
    OPERATOR_KEY=$(cat "$KEY_FILE")
    export OPERATOR_KEY
}

case "$MODE" in
    server)
        generate_key_if_missing
        exec /usr/local/bin/siqlah \
            --addr "${SIQLAH_ADDR:-:8080}" \
            --db "${SIQLAH_DB:-/data/siqlah.db}" \
            --operator-key "$OPERATOR_KEY" \
            --batch-interval "${SIQLAH_BATCH_INTERVAL:-30s}" \
            --max-batch "${SIQLAH_MAX_BATCH:-1000}" \
            --witnesses "${SIQLAH_WITNESSES:-}" \
            ${SIQLAH_MONITOR:+--monitor} \
            --monitor-interval "${SIQLAH_MONITOR_INTERVAL:-60s}" \
            --discrepancy-threshold "${SIQLAH_DISCREPANCY_THRESHOLD:-5.0}" \
            ${SIQLAH_ALERT_WEBHOOK:+--alert-webhook "$SIQLAH_ALERT_WEBHOOK"}
        ;;
    witness)
        KEY_FILE="${WITNESS_KEY_FILE:-/data/witness.key}"
        if [ ! -f "$KEY_FILE" ]; then
            echo "Generating witness keypair..."
            /usr/local/bin/witness keygen --out "$KEY_FILE"
            echo "Witness key written to $KEY_FILE"
        fi
        exec /usr/local/bin/witness watch \
            --ledger "${SIQLAH_LEDGER_URL:-http://siqlah:8080}" \
            --op-pub "${SIQLAH_OPERATOR_PUB:?SIQLAH_OPERATOR_PUB required for witness mode}" \
            --key "$KEY_FILE" \
            --interval "${WITNESS_INTERVAL:-30s}" \
            --witness-id "${WITNESS_ID:-}"
        ;;
    *)
        exec "$@"
        ;;
esac
