package objectPool

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoConnectionFactory implements ObjectFactory for MongoDB connections
type MongoConnectionFactory struct {
	uri string
}

func NewMongoConnectionFactory(uri string) *MongoConnectionFactory {
	return &MongoConnectionFactory{uri: uri}
}

func (f *MongoConnectionFactory) Create() (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(f.uri))
	if err != nil {
		return nil, fmt.Errorf("failed to create MongoDB client: %w", err)
	}
	return client, nil
}

func (f *MongoConnectionFactory) Destroy(client *mongo.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Disconnect(ctx); err != nil {
		return fmt.Errorf("failed to disconnect MongoDB client: %w", err)
	}
	return nil
}

func (f *MongoConnectionFactory) Validate(client *mongo.Client) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return client.Ping(ctx, nil) == nil
}
