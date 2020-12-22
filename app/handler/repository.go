package handler

import (
	"context"
	"fmt"
	"github.com/memochou1993/github-rankings/app"
	"github.com/memochou1993/github-rankings/app/model"
	"github.com/memochou1993/github-rankings/database"
	"github.com/memochou1993/github-rankings/logger"
	"github.com/memochou1993/github-rankings/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"strconv"
	"time"
)

type RepositoryHandler struct {
	BatchModel      *model.BatchModel
	RepositoryModel *model.RepositoryModel
}

func NewRepositoryHandler() *RepositoryHandler {
	return &RepositoryHandler{
		BatchModel:      model.NewBatchModel(),
		RepositoryModel: model.NewRepositoryModel(),
	}
}

func (r *RepositoryHandler) Init(starter chan<- struct{}) {
	logger.Info("Initializing repositories collection...")
	r.CreateIndexes()
	logger.Success("Repositories collection initialized!")
	starter <- struct{}{}
}

func (r *RepositoryHandler) Collect() error {
	logger.Info("Collecting repositories...")
	from := time.Date(2007, time.October, 1, 0, 0, 0, 0, time.UTC)
	q := model.NewRepositoriesQuery()

	return r.Travel(&from, q)
}

func (r *RepositoryHandler) Travel(from *time.Time, q *model.Query) error {
	to := time.Now()
	if from.After(to) {
		return nil
	}

	q.SearchArguments.Query = strconv.Quote(util.ParseStruct(r.searchQuery(*from), " "))

	var repositories []model.Repository
	if err := r.FetchRepositories(q, &repositories); err != nil {
		return err
	}
	r.StoreRepositories(repositories)
	*from = from.AddDate(0, 0, 7)

	return r.Travel(from, q)
}

func (r *RepositoryHandler) FetchRepositories(q *model.Query, repositories *[]model.Repository) error {
	res := model.RepositoryResponse{}
	if err := r.fetch(*q, &res); err != nil {
		return err
	}
	for _, edge := range res.Data.Search.Edges {
		*repositories = append(*repositories, edge.Node)
	}
	res.Data.RateLimit.Break()
	if !res.Data.Search.PageInfo.HasNextPage {
		q.SearchArguments.After = ""
		return nil
	}
	q.SearchArguments.After = strconv.Quote(res.Data.Search.PageInfo.EndCursor)

	return r.FetchRepositories(q, repositories)
}

func (r *RepositoryHandler) StoreRepositories(repositories []model.Repository) {
	if len(repositories) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var models []mongo.WriteModel
	for _, repository := range repositories {
		filter := bson.D{{"_id", repository.NameWithOwner}}
		update := bson.D{{"$set", repository}}
		models = append(models, mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update).SetUpsert(true))
	}
	res, err := r.RepositoryModel.Model.Collection().BulkWrite(ctx, models)
	if err != nil {
		log.Fatalln(err.Error())
	}
	if res.ModifiedCount > 0 {
		logger.Success(fmt.Sprintf("Updated %d repositories!", res.ModifiedCount))
	}
	if res.UpsertedCount > 0 {
		logger.Success(fmt.Sprintf("Inserted %d repositories!", res.UpsertedCount))
	}
}

func (r *RepositoryHandler) CreateIndexes() {
	database.CreateIndexes(r.RepositoryModel.Model.Name(), []string{
		"created_at",
		"ranks.tags",
	})
}

func (r *RepositoryHandler) fetch(q model.Query, res *model.RepositoryResponse) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.Fetch(ctx, fmt.Sprint(q), res); err != nil {
		return err
	}
	for _, err := range res.Errors {
		return err
	}
	return
}

func (r *RepositoryHandler) searchQuery(from time.Time) model.SearchQuery {
	return model.SearchQuery{
		Created: fmt.Sprintf("%s..%s", from.Format(time.RFC3339), from.AddDate(0, 0, 7).Format(time.RFC3339)),
		Fork:    "true",
		Sort:    "stars",
		Stars:   ">=100",
	}
}
