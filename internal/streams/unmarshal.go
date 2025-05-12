package streams

import (
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ConvertEventAttributeValueToDynamoDBAttributeValue converts a single Lambda event DDB attribute value
// to a DynamoDB service API attribute value.
func ConvertEventAttributeValueToDynamoDBAttributeValue(eventVal events.DynamoDBAttributeValue) (dynamodbtypes.AttributeValue, error) {
	switch eventVal.DataType() {
	case events.DataTypeString:
		return &dynamodbtypes.AttributeValueMemberS{Value: eventVal.String()}, nil
	case events.DataTypeNumber:
		return &dynamodbtypes.AttributeValueMemberN{Value: eventVal.Number()}, nil
	case events.DataTypeBinary:
		return &dynamodbtypes.AttributeValueMemberB{Value: eventVal.Binary()}, nil
	case events.DataTypeBoolean:
		return &dynamodbtypes.AttributeValueMemberBOOL{Value: eventVal.Boolean()}, nil
	case events.DataTypeMap:
		mapVal, err := ConvertEventStreamImageToDynamoDBItem(eventVal.Map())
		if err != nil {
			return nil, fmt.Errorf("error converting map attribute: %w", err)
		}
		return &dynamodbtypes.AttributeValueMemberM{Value: mapVal}, nil
	case events.DataTypeList:
		listVal := make([]dynamodbtypes.AttributeValue, len(eventVal.List()))
		for i, item := range eventVal.List() {
			convertedItem, err := ConvertEventAttributeValueToDynamoDBAttributeValue(item)
			if err != nil {
				return nil, fmt.Errorf("error converting list item at index %d: %w", i, err)
			}
			listVal[i] = convertedItem
		}
		return &dynamodbtypes.AttributeValueMemberL{Value: listVal}, nil
	case events.DataTypeNull:
		return &dynamodbtypes.AttributeValueMemberNULL{Value: eventVal.IsNull()}, nil
	case events.DataTypeStringSet:
		return &dynamodbtypes.AttributeValueMemberSS{Value: eventVal.StringSet()}, nil
	case events.DataTypeNumberSet:
		return &dynamodbtypes.AttributeValueMemberNS{Value: eventVal.NumberSet()}, nil
	case events.DataTypeBinarySet:
		return &dynamodbtypes.AttributeValueMemberBS{Value: eventVal.BinarySet()}, nil
	default:
		return nil, fmt.Errorf("unsupported attribute type: %v", eventVal.DataType())
	}
}

// ConvertEventStreamImageToDynamoDBItem converts a map of Lambda event DDB attribute values
// to a map of DynamoDB service API attribute values (an item).
func ConvertEventStreamImageToDynamoDBItem(eventImage map[string]events.DynamoDBAttributeValue) (map[string]dynamodbtypes.AttributeValue, error) {
	dynamoDBItem := make(map[string]dynamodbtypes.AttributeValue, len(eventImage))
	for k, v := range eventImage {
		convertedVal, err := ConvertEventAttributeValueToDynamoDBAttributeValue(v)
		if err != nil {
			return nil, fmt.Errorf("error converting attribute %s: %w", k, err)
		}
		dynamoDBItem[k] = convertedVal
	}
	return dynamoDBItem, nil
}

// UnmarshalEventStreamImage unmarshals a DynamoDB stream event image (NewImage or OldImage)
// into a target struct.
func UnmarshalEventStreamImage[T any](eventImage map[string]events.DynamoDBAttributeValue, out *T) error {
	if eventImage == nil {
		return fmt.Errorf("event image is nil")
	}
	dynamoDBItem, err := ConvertEventStreamImageToDynamoDBItem(eventImage)
	if err != nil {
		return fmt.Errorf("failed to convert event stream image to DynamoDB item: %w", err)
	}
	return attributevalue.UnmarshalMap(dynamoDBItem, out)
}
