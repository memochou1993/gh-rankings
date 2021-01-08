package model

import (
	"context"
	"github.com/memochou1993/github-rankings/database"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

type Repository struct {
	CreatedAt         time.Time `json:"createdAt" bson:"created_at"`
	Forks             Directory `json:"forks" bson:"forks"`
	Name              string    `json:"name" bson:"name"`
	NameWithOwner     string    `json:"nameWithOwner" bson:"_id"`
	OpenGraphImageUrl string    `json:"openGraphImageUrl" bson:"open_graph_image_url"`
	Owner             struct {
		Login string `json:"login" bson:"login"`
	} `json:"owner" bson:"owner"`
	PrimaryLanguage struct {
		Name string `json:"name" bson:"name"`
	} `json:"primaryLanguage" bson:"primary_language"`
	Stargazers Directory `json:"stargazers" bson:"stargazers"`
	Watchers   Directory `json:"watchers" bson:"watchers"`
	Rank       *Rank     `json:"rank" bson:"rank,omitempty"`
}

func (r *Repository) ID() string {
	return r.NameWithOwner
}

type RepositoryResponse struct {
	Data struct {
		Search struct {
			Edges []struct {
				Cursor string     `json:"cursor"`
				Node   Repository `json:"node"`
			} `json:"edges"`
			PageInfo `json:"pageInfo"`
		} `json:"search"`
		RateLimit `json:"rateLimit"`
	} `json:"data"`
	Errors []Error `json:"errors"`
}

type RepositoryModel struct {
	*Model
}

func (r *RepositoryModel) CreateIndexes() {
	database.CreateIndexes(r.Name(), []string{
		"ranks.tags",
	})
}

func (r *RepositoryModel) List(tags []string, timestamp time.Time, page int) *mongo.Cursor {
	ctx := context.Background()
	limit := 10
	pipeline := mongo.Pipeline{
		bson.D{
			{"$unwind", "$ranks"},
		},
		bson.D{
			{"$match", bson.D{
				{"$and", []bson.D{{
					{"ranks.tags", tags},
					{"ranks.updated_at", timestamp},
				}}},
			}},
		},
		bson.D{
			{"$addFields", bson.D{
				{"rank", "$ranks"},
			}},
		},
		bson.D{
			{"$project", bson.D{
				{"ranks", 0},
			}},
		},
		bson.D{
			{"$sort", bson.D{
				{"rank.rank", 1},
			}},
		},
		bson.D{
			{"$skip", (page - 1) * limit},
		},
		bson.D{
			{"$limit", limit},
		},
	}
	return database.Aggregate(ctx, r.Name(), pipeline)
}

func (r *RepositoryModel) Store(repositories []Repository) *mongo.BulkWriteResult {
	if len(repositories) == 0 {
		return nil
	}
	var models []mongo.WriteModel
	for _, repository := range repositories {
		filter := bson.D{{"_id", repository.ID()}}
		update := bson.D{{"$set", repository}}
		models = append(models, mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update).SetUpsert(true))
	}
	return database.BulkWrite(r.Name(), models)
}

func NewRepositoryModel() *RepositoryModel {
	return &RepositoryModel{
		&Model{
			name: "repositories",
		},
	}
}
