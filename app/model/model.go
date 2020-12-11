package model

import (
	"context"
	"github.com/memochou1993/github-rankings/database"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"time"
)

type CollectionInterface interface {
	GetCollection() *mongo.Collection
}

type Collection struct {
	collectionName string
}

func (c *Collection) SetCollectionName(collectionName string) {
	c.collectionName = collectionName
}

func (c *Collection) GetCollection() *mongo.Collection {
	return database.GetCollection(c.collectionName)
}

func (c *Collection) Count() int64 {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	count, err := database.Count(ctx, c.collectionName)
	if err != nil {
		log.Fatalln(err.Error())
	}

	return count
}
