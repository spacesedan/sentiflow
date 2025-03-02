#!/bin/bash

echo "Waiting for Kafka to be ready..."

MAX_RETRIES=20 # Increase retries to wait longer (20 * 5s = 100s max)
RETRY_COUNT=0

sleep 15

while ! /opt/bitnami/kafka/bin/kafka-topics.sh --bootstrap-server kafka:9092 --list &>/dev/null; do
    if [[ "$RETRY_COUNT" -ge "$MAX_RETRIES" ]]; then
        echo "Kafka did not become available within time limit. Exiting..."
        exit 1
    fi
    echo "Kafka is not available yet. Sleeping for 5 seconds..."
    sleep 5
    ((RETRY_COUNT++))
done

echo "Kafka is ready. Creating topic..."
/opt/bitnami/kafka/bin/kafka-topics.sh --bootstrap-server kafka:9092 \
    --create --if-not-exists --topic reddit-content --partitions 6 --replication-factor 1

echo "Kafka topic setup complete!"
