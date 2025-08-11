package main

import (
	objectPool "ObjectPool/pool"
	"context"
	"fmt"
	"log"

	//
	"time"
	//"go.mongodb.org/mongo-driver/bson"
	//"connectionpool/pool/objectPool"
)

func main() {
	// Configure pool
	config := objectPool.PoolConfig{
		MaxSize:      10,
		InitialSize:  3,
		IdleTimeout:  5 * time.Minute,
		WaitTimeout:  5 * time.Second,
		TestOnBorrow: true,
	}

	// Create pool
	pool, err := objectPool.NewObjectPool(config, objectPool.NewMongoConnectionFactory("mongodb://localhost:27017"))
	if err != nil {
		log.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	for i := 0; i < 200; i++ {
		// Get a connection
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		client, err := pool.Get(ctx)
		if err != nil {
			log.Fatalf("Failed to get client: %v", err)
		}

		/*// Use the connection for a sample operation
		coll := client.Database("test").Collection("users")
		_, err = coll.InsertOne(ctx, bson.M{"name": "sandeep", "age": 30})
		if err != nil {
			log.Printf("Failed to insert document: %v", err)
			_ = pool.Put(client)
			return
		}

		// Query the inserted document
		var result bson.M
		err = coll.FindOne(ctx, bson.M{"name": "sandeep"}).Decode(&result)
		if err != nil {
			log.Printf("Failed to query document: %v", err)
			_ = pool.Put(client)
			return
		}
		fmt.Printf("Found document: %+v\n", result)*/

		// Return connection to pool
		if err := pool.Put(client); err != nil {
			log.Printf("Failed to return client to pool: %v", err)
		}
		log.Printf("count: %v", i)
		// Check pool stats
		available, total := pool.Stats()
		fmt.Printf("Pool stats - Available: %d, Total: %d\n", available, total)
	}
}
