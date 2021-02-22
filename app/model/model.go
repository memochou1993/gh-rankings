package model

import (
	"context"
	"github.com/memochou1993/gh-rankings/database"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
)

type Interface interface {
	Name() string
	Collection() *mongo.Collection
}

type Model struct {
	name string
}

func (m *Model) Name() string {
	return m.name
}

func (m *Model) Collection() *mongo.Collection {
	return database.Collection(m.name)
}

func (m *Model) List(filter bson.D, v interface{}) {
	cursor := database.Find(m.Name(), filter)
	if err := cursor.All(context.Background(), v); err != nil {
		log.Fatal(err.Error())
	}
}

func (m *Model) FindByName(name string, v interface{}) {
	res := database.FindOne(m.Name(), bson.D{{"_id", name}})
	if err := res.Decode(v); err != nil && err != mongo.ErrNoDocuments {
		log.Fatal(err.Error())
	}
}

func (m *Model) Last(v interface{}) {
	opts := options.FindOne().SetSort(bson.D{{"$natural", -1}})
	res := database.FindOne(m.Name(), bson.D{}, opts)
	if err := res.Decode(v); err != nil && err != mongo.ErrNoDocuments {
		log.Fatal(err.Error())
	}
}
