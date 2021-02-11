package model

import (
	"fmt"
	"github.com/memochou1993/gh-rankings/database"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"time"
)

type Organization struct {
	AvatarURL    string       `json:"avatarUrl,omitempty" bson:"avatar_url,omitempty"`
	CreatedAt    *time.Time   `json:"createdAt,omitempty" bson:"created_at,omitempty"`
	Location     string       `json:"location,omitempty" bson:"location,omitempty"`
	Login        string       `json:"login" bson:"_id"`
	Name         string       `json:"name,omitempty" bson:"name,omitempty"`
	Repositories []Repository `json:"repositories,omitempty" bson:"repositories,omitempty"`
	Tags         []string     `json:"tags,omitempty" bson:"tags,omitempty"`
}

func (o *Organization) ID() string {
	return o.Login
}

func (o *Organization) AddTypeTag() {
	o.Tags = append(o.Tags, fmt.Sprintf("type:%s", TypeOrganization))
}

func (o *Organization) AddLocationTag() {
	// for _, location := range resource.Locate(o.Location) {
	// 	o.Tags = append(o.Tags, fmt.Sprintf("location:%s", location))
	// }
}

type OrganizationModel struct {
	*Model
}

func (o *OrganizationModel) FindLast() (organization Organization) {
	opts := options.FindOne().SetSort(bson.D{{"$natural", -1}})
	res := database.FindOne(o.Name(), bson.D{}, opts)
	if err := res.Decode(&organization); err != nil && err != mongo.ErrNoDocuments {
		log.Fatal(err.Error())
	}
	return
}

func (o *OrganizationModel) Store(organizations []Organization) *mongo.BulkWriteResult {
	if len(organizations) == 0 {
		return nil
	}
	var models []mongo.WriteModel
	for _, organization := range organizations {
		organization.AddTypeTag()
		organization.AddLocationTag()
		filter := bson.D{{"_id", organization.ID()}}
		update := bson.D{{"$set", organization}}
		models = append(models, mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update).SetUpsert(true))
	}
	return database.BulkWrite(o.Name(), models)
}

func (o *OrganizationModel) UpdateRepositories(organization Organization, repositories []Repository) {
	filter := bson.D{{"_id", organization.ID()}}
	update := bson.D{{"$set", bson.D{{"repositories", repositories}}}}
	database.UpdateOne(o.Name(), filter, update)
}

func NewOrganizationModel() *OrganizationModel {
	return &OrganizationModel{
		&Model{
			name: "organizations",
		},
	}
}
