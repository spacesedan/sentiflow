DOCKER_DIR=./deploy/docker
DYNAMODB_PORT=8000
DYNAMODB_ENDPOINT=http://localhost:8000
AWS_REGION=us-west-2

DOCKER_COMPOSE_FILE=$(DOCKER_DIR)/docker-compose.yml

# Docker compose start stop services
.PHONY: start_services
start_services:
	docker compose -f $(DOCKER_COMPOSE_FILE) up -d

.PHONY: stop_services
stop_services:
	docker compose -f $(DOCKER_COMPOSE_FILE) down

.PHONY: refresh_services
refresh_services:
	docker compose -f $(DOCKER_COMPOSE_FILE) down -v --remove-orphans

.PHONY: restart_services
restart_services: refresh_services start_services

.PHONY: service_logs
service_logs:
	@read -p "Enter the service name: " service; \
	if [ -z "$$service" ]; then \
		echo "ERROR: SERVICE is required!"; \
		exit 1; \
	fi; \
	docker compose -f $(DOCKER_COMPOSE_FILE) logs -f $$service

.PHONY: restart_image
restart_image:
	@read -p "Enter the service name: " service; \
	if [ -z "$$service" ]; then \
		echo "ERROR: SERVICE is required!"; \
		exit 1; \
	fi; \
	docker compose -f $(DOCKER_COMPOSE_FILE) restart $$service
# DynamoDB
#
TOPICS_TABLE_NAME=Topics


.PHONY: init_topics_table
init_topics_table:
	@echo "Creating '$(TOPICS_TABLE_NAME)' table in local DynamoDB..."
	aws dynamodb create-table \
		--table-name $(TOPICS_TABLE_NAME) \
		--attribute-definitions \
			AttributeName=url,AttributeType=S \
		--key-schema \
			AttributeName=url,KeyType=HASH \
		--billing-mode PAY_PER_REQUEST \
		--region $(AWS_REGION) \
		--endpoint-url $(DYNAMODB_ENDPOINT)

.PHONY: update_topics_table_ttl
update_topics_table_ttl:
	@echo "Enabling TTL attribute to '$(TOPICS_TABLE_NAME)'..."
	aws dynamodb update-time-to-live \
    --table-name $(TOPICS_TABLE_NAME) \
    --time-to-live-specification "Enabled=true, AttributeName=expires_at" \
    --region $(AWS_REGION) \
    --endpoint-url $(DYNAMODB_ENDPOINT)

.PHONY: create_topics_table
create_topics_table: init_topics_table update_topics_table_ttl


.PHONY: list_tables
list_tables:
	@echo "Listing all tables in DynamoDB..."
	aws dynamodb list-tables --region $(AWS_REGION) --endpoint-url $(DYNAMODB_ENDPOINT)

.PHONY: describe_topics_table
describe_topics_table:
	@echo "Describing the '$(TOPICS_TABLE_NAME)' table..."
	aws dynamodb describe-table --table-name $(TOPICS_TABLE_NAME) --region $(AWS_REGION) --endpoint-url $(DYNAMODB_ENDPOINT)

# Seed Sample Data
.PHONY: seed_data
seed_data:
	@echo "Seeding sample data into '$(TOPICS_TABLE_NAME)'..."
	aws dynamodb put-item \
	    --table-name $(TOPICS_TABLE_NAME) \
		--item "{ \
			\"url\": {\"S\": \"https://example.com/article1\"}, \
			\"category\": {\"S\": \"Technology\"}, \
			\"title\": {\"S\": \"AI Breakthrough in 2025\"} \
		}" \
		--region $(AWS_REGION) \
		--endpoint-url $(DYNAMODB_ENDPOINT)

# Query a Specific Item
.PHONY: query_item
query_item:
	@echo "Querying a specific topic from '$(TOPICS_TABLE_NAME)'..."
	aws dynamodb get-item \
	    --table-name $(TOPICS_TABLE_NAME) \
	    --key '{"url": {"S": "https://example.com/article1"}}' \
	    --region $(AWS_REGION) \
	    --endpoint-url $(DYNAMODB_ENDPOINT)

# Delete All Tables
.PHONY: delete_table
delete_table:
	@echo "Deleting table '$(TOPICS_TABLE_NAME)'..."
	aws dynamodb delete-table --table-name $(TOPICS_TABLE_NAME) --region $(AWS_REGION) --endpoint-url $(DYNAMODB_ENDPOINT)

# Wipe Local DynamoDB Data (Delete & Recreate Table)
.PHONY: reset_dynamodb
reset_dynamodb: delete_table create_topics_table

.PHONY: update_kafka_partitions
update_kafka_partitions:
	docker compose -f $(DOCKER_COMPOSE_FILE) exec kafka \
		/opt/bitnami/kafka/bin/kafka-topics.sh --bootstrap-server kafka:9092 \
		--create --if-not-exists --topic sentiment-request --partitions 6 --replication-factor 1
