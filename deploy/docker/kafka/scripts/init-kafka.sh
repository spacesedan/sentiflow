#!/bin/bash

set -e
echo "[init-kafka.sh] Creating Kafka topics..."

kafka-topics.sh --bootstrap-server kafka:9092 --create --if-not-exists \
    --topic raw-content --partitions 6 --replication-factor 1

kafka-topics.sh --bootstrap-server kafka:9092 --create --if-not-exists \
    --topic summary-request --partitions 3 --replication-factor 1

kafka-topics.sh --bootstrap-server kafka:9092 --create --if-not-exists \
    --topic sentiment-request --partitions 3 --replication-factor 1

kafka-topics.sh --bootstrap-server kafka:9092 --create --if-not-exists \
    --topic sentiment-results --partitions 3 --replication-factor 1

echo "[init-kafka.sh] Topic creation completed."
