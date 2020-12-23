package model

import (
	"github.com/memochou1993/github-rankings/database"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

type Owner struct {
	AvatarURL    string       `json:"avatarUrl" bson:"avatar_url"`
	CreatedAt    time.Time    `json:"createdAt" bson:"created_at"`
	Followers    *Directory   `json:"followers" bson:"followers"`
	Location     string       `json:"location" bson:"location"`
	Login        string       `json:"login" bson:"_id"`
	Name         string       `json:"name" bson:"name"`
	Gists        []Gist       `json:"gists" bson:"gists,omitempty"`
	Repositories []Repository `json:"repositories" bson:"repositories,omitempty"`
	Ranks        []Rank       `json:"ranks" bson:"ranks,omitempty"`
	Type         string       `json:"type" bson:"type"`
}

func (o *Owner) ID() string {
	return o.Login
}

type OwnerResponse struct {
	Data struct {
		Search struct {
			Edges []struct {
				Cursor string `json:"cursor"`
				Node   Owner  `json:"node"`
			} `json:"edges"`
			PageInfo `json:"pageInfo"`
		} `json:"search"`
		Owner struct {
			AvatarURL string    `json:"avatarUrl"`
			CreatedAt time.Time `json:"createdAt"`
			Followers Directory `json:"followers"`
			Gists     struct {
				Edges []struct {
					Cursor string `json:"cursor"`
					Node   Gist   `json:"node"`
				} `json:"edges"`
				PageInfo `json:"pageInfo"`
			} `json:"gists"`
			Location     string `json:"location"`
			Login        string `json:"login"`
			Name         string `json:"name"`
			Repositories struct {
				Edges []struct {
					Cursor string     `json:"cursor"`
					Node   Repository `json:"node"`
				} `json:"edges"`
				PageInfo `json:"pageInfo"`
			} `json:"repositories"`
		} `json:"owner"`
		RateLimit `json:"rateLimit"`
	} `json:"data"`
	Errors []Error `json:"errors"`
}

type OwnerModel struct {
	*Model
}

func NewOwnerModel() *OwnerModel {
	return &OwnerModel{
		&Model{
			name: "owners",
		},
	}
}

func (o *OwnerModel) CreateIndexes() {
	database.CreateIndexes(o.Name(), []string{
		"created_at",
		"name",
		"ranks.tags",
	})
}

func (o *OwnerModel) Store(owners []Owner) *mongo.BulkWriteResult {
	if len(owners) == 0 {
		return nil
	}
	var models []mongo.WriteModel
	for _, owner := range owners {
		owner.Type = o.Type(owner)
		filter := bson.D{{"_id", owner.ID()}}
		update := bson.D{{"$set", owner}}
		models = append(models, mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update).SetUpsert(true))
	}
	return database.BulkWrite(o.Name(), models)
}

func (o *OwnerModel) UpdateGists(owner Owner, gists []Gist) {
	filter := bson.D{{"_id", owner.ID()}}
	update := bson.D{{"$set", bson.D{{"gists", gists}}}}
	database.UpdateOne(o.Name(), filter, update)
}

func (o *OwnerModel) UpdateRepositories(owner Owner, repositories []Repository) {
	filter := bson.D{{"_id", owner.ID()}}
	update := bson.D{{"$set", bson.D{{"repositories", repositories}}}}
	database.UpdateOne(o.Name(), filter, update)
}

func (o *OwnerModel) IsUser(owner Owner) bool {
	return o.Type(owner) == TypeUser
}

func (o *OwnerModel) IsOrganization(owner Owner) bool {
	return o.Type(owner) == TypeOrganization
}

func (o *OwnerModel) Type(owner Owner) (ownerType string) {
	ownerType = TypeUser
	if owner.Followers == nil {
		ownerType = TypeOrganization
	}
	return
}
