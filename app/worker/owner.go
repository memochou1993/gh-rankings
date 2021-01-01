package worker

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
	"os"
	"strconv"
	"sync"
	"time"
)

type OwnerWorker struct {
	OwnerModel *model.OwnerModel
	UpdatedAt  time.Time
}

func NewOwnerWorker() *OwnerWorker {
	return &OwnerWorker{
		OwnerModel: model.NewOwnerModel(),
	}
}

func (o *OwnerWorker) Init() {
	logger.Info("Initializing owner collection...")
	o.OwnerModel.CreateIndexes()
	logger.Success("Owner collection initialized!")
}

func (o *OwnerWorker) Collect() error {
	logger.Info("Collecting owners...")
	from := time.Date(2007, time.October, 1, 0, 0, 0, 0, time.UTC)
	q := model.NewOwnersQuery()

	return o.Travel(&from, q)
}

func (o *OwnerWorker) Travel(from *time.Time, q *model.Query) error {
	to := time.Now()
	if from.After(to) {
		return nil
	}

	q.SearchArguments.Query = strconv.Quote(util.ParseStruct(o.newSearchQuery(*from), " "))

	var owners []model.Owner
	if err := o.FetchOwners(q, &owners); err != nil {
		return err
	}
	if res := o.OwnerModel.Store(owners); res != nil {
		if res.ModifiedCount > 0 {
			logger.Success(fmt.Sprintf("Updated %d owners!", res.ModifiedCount))
		}
		if res.UpsertedCount > 0 {
			logger.Success(fmt.Sprintf("Inserted %d owners!", res.UpsertedCount))
		}
	}
	*from = from.AddDate(0, 0, 7)

	return o.Travel(from, q)
}

func (o *OwnerWorker) FetchOwners(q *model.Query, owners *[]model.Owner) error {
	res := model.OwnerResponse{}
	if err := o.fetch(*q, &res); err != nil {
		return err
	}
	for _, edge := range res.Data.Search.Edges {
		*owners = append(*owners, edge.Node)
	}
	res.Data.RateLimit.Break()
	if !res.Data.Search.PageInfo.HasNextPage {
		q.SearchArguments.After = ""
		return nil
	}
	q.SearchArguments.After = strconv.Quote(res.Data.Search.PageInfo.EndCursor)

	return o.FetchOwners(q, owners)
}

func (o *OwnerWorker) Update() error {
	ctx := context.Background()
	cursor := database.All(ctx, o.OwnerModel.Name())
	defer database.CloseCursor(ctx, cursor)

	if cursor.RemainingBatchLength() == 0 {
		return nil
	}
	logger.Info("Updating user gists...")
	gistsQuery := model.NewOwnerGistsQuery()
	logger.Info("Updating owner repositories...")
	repositoriesQuery := model.NewOwnerRepositoriesQuery()
	for cursor.Next(context.Background()) {
		owner := model.Owner{}
		if err := cursor.Decode(&owner); err != nil {
			log.Fatalln(err.Error())
		}

		if owner.IsUser() {
			var gists []model.Gist
			gistsQuery.OwnerArguments.Login = strconv.Quote(owner.ID())
			if err := o.FetchGists(gistsQuery, &gists); err != nil {
				return err
			}
			o.OwnerModel.UpdateGists(owner, gists)
			logger.Success(fmt.Sprintf("Updated %d user gists!", len(gists)))
		}

		var repositories []model.Repository
		repositoriesQuery.Field = owner.Type()
		repositoriesQuery.OwnerArguments.Login = strconv.Quote(owner.ID())
		if err := o.FetchRepositories(repositoriesQuery, &repositories); err != nil {
			return err
		}
		o.OwnerModel.UpdateRepositories(owner, repositories)
		logger.Success(fmt.Sprintf("Updated %d %s repositories!", len(repositories), owner.Type()))
	}

	return nil
}

func (o *OwnerWorker) FetchGists(q *model.Query, gists *[]model.Gist) error {
	res := model.OwnerResponse{}
	if err := o.fetch(*q, &res); err != nil {
		return err
	}
	for _, edge := range res.Data.Owner.Gists.Edges {
		*gists = append(*gists, edge.Node)
	}
	res.Data.RateLimit.Break()
	if !res.Data.Owner.Gists.PageInfo.HasNextPage {
		q.GistsArguments.After = ""
		return nil
	}
	q.GistsArguments.After = strconv.Quote(res.Data.Owner.Gists.PageInfo.EndCursor)

	return o.FetchGists(q, gists)
}

func (o *OwnerWorker) FetchRepositories(q *model.Query, repositories *[]model.Repository) error {
	res := model.OwnerResponse{}
	if err := o.fetch(*q, &res); err != nil {
		return err
	}
	for _, edge := range res.Data.Owner.Repositories.Edges {
		*repositories = append(*repositories, edge.Node)
	}
	res.Data.RateLimit.Break()
	if !res.Data.Owner.Repositories.PageInfo.HasNextPage {
		q.RepositoriesArguments.After = ""
		return nil
	}
	q.RepositoriesArguments.After = strconv.Quote(res.Data.Owner.Repositories.PageInfo.EndCursor)

	return o.FetchRepositories(q, repositories)
}

func (o *OwnerWorker) Rank() {
	logger.Info("Executing owner rank pipelines...")
	pipelines := []*model.RankPipeline{
		o.newRankPipeline(model.TypeUser, "followers"),
		o.newRankPipeline(model.TypeUser, "gists.forks"),
		o.newRankPipeline(model.TypeUser, "gists.stargazers"),
		o.newRankPipeline(model.TypeUser, "repositories.forks"),
		o.newRankPipeline(model.TypeUser, "repositories.stargazers"),
		o.newRankPipeline(model.TypeUser, "repositories.watchers"),
		o.newRankPipeline(model.TypeOrganization, "repositories.forks"),
		o.newRankPipeline(model.TypeOrganization, "repositories.stargazers"),
		o.newRankPipeline(model.TypeOrganization, "repositories.watchers"),
	}
	pipelines = append(pipelines, o.newRepositoryRankPipelinesByLanguage(model.TypeUser, "forks")...)
	pipelines = append(pipelines, o.newRepositoryRankPipelinesByLanguage(model.TypeUser, "stargazers")...)
	pipelines = append(pipelines, o.newRepositoryRankPipelinesByLanguage(model.TypeUser, "watchers")...)
	pipelines = append(pipelines, o.newRepositoryRankPipelinesByLanguage(model.TypeOrganization, "forks")...)
	pipelines = append(pipelines, o.newRepositoryRankPipelinesByLanguage(model.TypeOrganization, "stargazers")...)
	pipelines = append(pipelines, o.newRepositoryRankPipelinesByLanguage(model.TypeOrganization, "watchers")...)

	ch := make(chan struct{}, 4)
	wg := sync.WaitGroup{}
	wg.Add(len(pipelines))
	updatedAt := time.Now()
	for _, pipeline := range pipelines {
		ch <- struct{}{}
		go func(pipeline *model.RankPipeline) {
			defer wg.Done()
			model.PushRanks(o.OwnerModel, updatedAt, *pipeline)
			<-ch
		}(pipeline)
	}
	wg.Wait()
	o.UpdatedAt = updatedAt
	model.PullRanks(o.OwnerModel, updatedAt)
	logger.Success(fmt.Sprintf("Executed %d owner rank pipelines!", len(pipelines)))
}

func (o *OwnerWorker) fetch(q model.Query, res *model.OwnerResponse) (err error) {
	if err := app.Fetch(context.Background(), fmt.Sprint(q), res); err != nil {
		if os.IsTimeout(err) {
			logger.Error("Retrying...")
			return o.fetch(q, res)
		}
		return err
	}
	for _, err := range res.Errors {
		return err
	}
	return
}

func (o *OwnerWorker) newSearchQuery(from time.Time) *model.SearchQuery {
	return &model.SearchQuery{
		Created: fmt.Sprintf("%s..%s", from.Format(time.RFC3339), from.AddDate(0, 0, 7).Format(time.RFC3339)),
		Repos:   ">=5",
		Sort:    "joined-asc",
	}
}

func (o *OwnerWorker) newRankPipeline(tag string, field string) *model.RankPipeline {
	return &model.RankPipeline{
		Pipeline: &mongo.Pipeline{
			bson.D{
				{"$match", bson.D{
					{"tags", tag},
				}},
			},
			bson.D{
				{"$project", bson.D{
					{"_id", "$_id"},
					{"total_count", bson.D{
						{"$sum", fmt.Sprintf("$%s.total_count", field)},
					}},
				}},
			},
			bson.D{
				{"$sort", bson.D{
					{"total_count", -1},
				}},
			},
		},
		Tags: []string{tag, field},
	}
}

func (o *OwnerWorker) newRepositoryRankPipelinesByLanguage(tag string, field string) (pipelines []*model.RankPipeline) {
	for _, language := range languages {
		pipelines = append(pipelines, &model.RankPipeline{
			Pipeline: &mongo.Pipeline{
				bson.D{
					{"$match", bson.D{
						{"tags", tag},
					}},
				},
				bson.D{
					{"$unwind", "$repositories"},
				},
				bson.D{
					{"$match", bson.D{
						{"repositories.primary_language.name", language.Name},
					}},
				},
				bson.D{
					{"$group", bson.D{
						{"_id", "$_id"},
						{"total_count", bson.D{
							{"$sum", fmt.Sprintf("$repositories.%s.total_count", field)},
						}},
					}},
				},
				bson.D{
					{"$sort", bson.D{
						{"total_count", -1},
					}},
				},
			},
			Tags: []string{tag, fmt.Sprintf("repositories.%s", field), language.Name},
		})
	}
	return
}
