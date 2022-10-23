package repository

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/deloittepark/apigw-lambda-url-shortener/internal/client"
	"github.com/deloittepark/apigw-lambda-url-shortener/internal/entities"
	"go.uber.org/zap"
)

// Repository interface allows us to access the CRUD Operations in dynamodb.
type DynamoDbRepository interface {
	CreateUrl(url *entities.Url) (*entities.Url, error)
	GetOriginalUrl(path *string) (*entities.Url, error)
}

type dynamoDbRepository struct {
	table  *string
	client *dynamodb.Client
}

// NewRepo is the single instance repo that is being created.
func NewRepository(client *client.Client) DynamoDbRepository {
	return &dynamoDbRepository{
		table:  client.Table,
		client: client.DynamoDbClient,
	}
}

func (r *dynamoDbRepository) GetOriginalUrl(path *string) (*entities.Url, error) {
	zap.L().Debug("Repository GetOriginalUrl starts process")
	var (
		item         = new(entities.Url)
		keyCondition = expression.KeyAnd(
			expression.Key(entities.GSI_PK).Equal(expression.Value(entities.Type)),
			expression.Key(entities.GSI_SK).Equal(expression.Value(entities.STATE_ACTVE)),
		)
		filterExpr = expression.Equal(expression.Name("Path"), expression.Value(path))
		expr, _    = expression.NewBuilder().WithKeyCondition(keyCondition).WithFilter(filterExpr).Build()
	)

	queryInput := &dynamodb.QueryInput{
		TableName:                 r.table,
		IndexName:                 aws.String(entities.GSI_INDEX),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
	}

	output, err := r.client.Query(context.TODO(), queryInput)
	if err != nil {
		return nil, err
	}

	err = attributevalue.UnmarshalMap(output.Items[0], item)
	if err != nil {
		return nil, err
	}
	zap.L().Debug("updating hitcount", zap.Any("url", item.OriginalUrl))
	r.updateHitCount(item)
	return item, nil
}

// CreateUrl is a dynamodb repository that helps to create new url entry
func (r *dynamoDbRepository) CreateUrl(entity *entities.Url) (*entities.Url, error) {
	zap.L().Debug("Repository CreateUrl starts process")
	var (
		item   = new(entities.Url)
		update = expression.Set(
			expression.Name("OriginalUrl"), expression.Value(entity.OriginalUrl),
		).Set(
			expression.Name("Path"), expression.Value(entity.Path),
		).Set(
			expression.Name("CreatedAt"), expression.Value(entity.CreatedAt),
		).Set(
			expression.Name("HitCount"), expression.Value(entity.HitCount),
		).Set(
			expression.Name("Type"), expression.Value(entity.Type),
		).Set(
			expression.Name("State"), expression.Value(entity.State),
		)
		expr, _         = expression.NewBuilder().WithUpdate(update).Build()
		updateItemInput = &dynamodb.UpdateItemInput{
			TableName: r.table,
			Key: map[string]types.AttributeValue{
				entities.PK: &types.AttributeValueMemberS{Value: entity.Id},
			},
			ReturnValues:              types.ReturnValueAllNew,
			UpdateExpression:          expr.Update(),
			ExpressionAttributeValues: expr.Values(),
			ExpressionAttributeNames:  expr.Names(),
		}
	)
	output, err := r.client.UpdateItem(context.TODO(), updateItemInput)
	if err != nil {
		return nil, err
	}

	err = attributevalue.UnmarshalMap(output.Attributes, item)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *dynamoDbRepository) updateHitCount(entity *entities.Url) {
	zap.L().Debug("Repository updateHitCount starts process")
	var (
		update = expression.Set(
			expression.Name("HitCount"), expression.Value(entity.HitCount+1),
		)
		expr, _         = expression.NewBuilder().WithUpdate(update).Build()
		updateItemInput = &dynamodb.UpdateItemInput{
			TableName: r.table,
			Key: map[string]types.AttributeValue{
				entities.PK: &types.AttributeValueMemberS{Value: entity.Id},
			},
			ReturnValues:              types.ReturnValueAllNew,
			UpdateExpression:          expr.Update(),
			ExpressionAttributeValues: expr.Values(),
			ExpressionAttributeNames:  expr.Names(),
		}
	)
	_, err := r.client.UpdateItem(context.TODO(), updateItemInput)
	if err != nil {
		zap.L().Error("unexpected error during updating hitcount", zap.Error(err))
	}
}
