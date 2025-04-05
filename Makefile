DOCKER_DIR=./deploy/docker
DYNAMODB_PORT=8000
DYNAMODB_ENDPOINT=http://localhost:8000
AWS_REGION=us-west-2

DOCKER_COMPOSE_FILE=$(DOCKER_DIR)/docker-compose.yml

# Docker compose start stop services
.PHONY: start_services
start_services:
	docker compose -f $(DOCKER_COMPOSE_FILE) up -d


.PHONY: start_services_attached
start_services_attached:
	docker compose -f $(DOCKER_COMPOSE_FILE) up

.PHONY: stop_services
stop_services:
	docker compose -f $(DOCKER_COMPOSE_FILE) down

.PHONY: build_services
build_services:
	docker compose -f $(DOCKER_COMPOSE_FILE) build --pull --no-cache

.PHONY: refresh_services
refresh_services:
	docker compose -f $(DOCKER_COMPOSE_FILE) down --remove-orphans

.PHONY: refresh_services_v
refresh_services_v:
	docker compose -f $(DOCKER_COMPOSE_FILE) down -v --remove-orphans

.PHONY: restart_services
restart_services: refresh_services start_services

.PHONY: restart_services_v
restart_services_v: refresh_services_v start_services

.PHONY: restart_services_attached
restart_services_attached: refresh_services start_services_attached

.PHONY: restart_services_attached_v
restart_services_attached_v: refresh_services_v start_services_attached

.PHONY: service_logs
service_logs:
	@read -p "Enter the service name: " service; \
	if [ -z "$$service" ]; then \
		echo "ERROR: SERVICE is required!"; \
		exit 1; \
	fi; \
	docker compose -f $(DOCKER_COMPOSE_FILE) logs -f $$service

.PHONY: service_shell
service_shell:
	@read -p "Enter the service name: " service; \
	if [ -z "$$service" ]; then \
		echo "ERROR: SERVICE is required!"; \
		exit 1; \
	fi; \
	docker compose -f $(DOCKER_COMPOSE_FILE) exec $$service sh -c "bash || sh"

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


.PHONY: update_topics_table_enable_streams
update_topics_table_enable_streams:
	@echo "Enabling Event Streaming on '$(TOPICS_TABLE_NAME)' ... "
	aws dynamodb update-table \
		--table-name $(TOPICS_TABLE_NAME) \
		--stream-specification "StreamEnabled=true, StreamViewType=NEW_IMAGE" \
		--region $(AWS_REGION) \
		--endpoint-url $(DYNAMODB_ENDPOINT)

.PHONY: create_topics_table
create_topics_table: init_topics_table update_topics_table_ttl update_topics_table_enable_streams
SENTIMENT_ANALYSIS_TABLE_NAME=SentimentResults

.PHONY: init_sentiment_table
init_sentiment_table:
	@echo "Creating '$(SENTIMENT_ANALYSIS_TABLE_NAME)' table in local DynamoDB..."
	aws dynamodb create-table \
		--table-name $(SENTIMENT_ANALYSIS_TABLE_NAME) \
		--attribute-definitions \
			AttributeName=content_id,AttributeType=S \
		--key-schema \
			AttributeName=content_id,KeyType=HASH \
		--billing-mode PAY_PER_REQUEST \
		--region $(AWS_REGION) \
		--endpoint-url $(DYNAMODB_ENDPOINT)

.PHONY: update_sentiment_table_ttl
update_sentiment_table_ttl:
	@echo "Enabling TTL attribute on '$(SENTIMENT_ANALYSIS_TABLE_NAME)'..."
	aws dynamodb update-time-to-live \
		--table-name $(SENTIMENT_ANALYSIS_TABLE_NAME) \
		--time-to-live-specification "Enabled=true, AttributeName=ttl" \
		--region $(AWS_REGION) \
		--endpoint-url $(DYNAMODB_ENDPOINT)

.PHONY: update_sentiment_table_enable_streams
update_sentiment_table_enable_streams:
	@echo "Enabling Event Streaming on '$(SENTIMENT_ANALYSIS_TABLE_NAME)' ... "
	aws dynamodb update-table \
		--table-name $(SENTIMENT_ANALYSIS_TABLE_NAME) \
		--stream-specification "StreamEnabled=true, StreamViewType=NEW_IMAGE" \
		--region $(AWS_REGION) \
		--endpoint-url $(DYNAMODB_ENDPOINT)

.PHONY: create_sentiment_table
create_sentiment_table: init_sentiment_table update_sentiment_table_ttl update_sentiment_table_enable_streams

.PHONY: create_tables
create_tables: create_topics_table create_sentiment_table

.PHONY: list_tables
list_tables:
	@echo "Listing all tables in DynamoDB..."
	aws dynamodb list-tables --region $(AWS_REGION) --endpoint-url $(DYNAMODB_ENDPOINT)

.PHONY: describe_topics_table
describe_topics_table:
	@echo "Describing the '$(TOPICS_TABLE_NAME)' table..."
	aws dynamodb describe-table --table-name $(TOPICS_TABLE_NAME) --region $(AWS_REGION) --endpoint-url $(DYNAMODB_ENDPOINT)

.PHONY: describe_sentiment_table
describe_sentiment_table:
	@echo "Describing the '$(SENTIMENT_ANALYSIS_TABLE_NAME)' table..."
	aws dynamodb describe-table --table-name $(SENTIMENT_ANALYSIS_TABLE_NAME) --region $(AWS_REGION) --endpoint-url $(DYNAMODB_ENDPOINT)

# Delete All Tables
.PHONY: delete_topics_table
delete_topics_table:
	@echo "Deleting table '$(TOPICS_TABLE_NAME)'..."
	aws dynamodb delete-table --table-name $(TOPICS_TABLE_NAME) --region $(AWS_REGION) --endpoint-url $(DYNAMODB_ENDPOINT)

.PHONY: delete_sentiment_table
delete_sentiment_table:
	@echo "Deleting table '$(SENTIMENT_ANALYSIS_TABLE_NAME)'..."
	aws dynamodb delete-table --table-name $(SENTIMENT_ANALYSIS_TABLE_NAME) --region $(AWS_REGION) --endpoint-url $(DYNAMODB_ENDPOINT)

.PHONY: delete_tables
delete_tables: delete_topics_table delete_sentiment_table

.PHONY: reset_tables
reset_tables: delete_tables create_tables

