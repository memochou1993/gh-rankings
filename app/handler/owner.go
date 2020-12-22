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
	"strings"
	"sync"
	"time"
)

const (
	typeUser         = "user"
	typeOrganization = "organization"
)

type OwnerHandler struct {
	BatchModel *model.BatchModel
	OwnerModel *model.OwnerModel
}

func NewOwnerHandler() *OwnerHandler {
	return &OwnerHandler{
		BatchModel: model.NewBatchModel(),
		OwnerModel: model.NewOwnerModel(),
	}
}

func (o *OwnerHandler) Init(starter chan<- struct{}) {
	logger.Info("Initializing owner collection...")
	o.CreateIndexes()
	logger.Success("Owner collection initialized!")
	starter <- struct{}{}
}

func (o *OwnerHandler) Collect() error {
	logger.Info("Collecting owners...")
	from := time.Date(2007, time.October, 1, 0, 0, 0, 0, time.UTC)
	q := model.NewSearchOwnersQuery()

	return o.Travel(&from, q)
}

func (o *OwnerHandler) Travel(from *time.Time, q *model.Query) error {
	to := time.Now()
	if from.After(to) {
		return nil
	}

	q.SearchArguments.Query = strconv.Quote(util.ParseStruct(o.searchQuery(*from), " "))

	var owners []model.Owner
	if err := o.FetchOwners(q, &owners); err != nil {
		return err
	}
	o.StoreOwners(owners)
	*from = from.AddDate(0, 0, 7)

	return o.Travel(from, q)
}

func (o *OwnerHandler) FetchOwners(q *model.Query, owners *[]model.Owner) error {
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

func (o *OwnerHandler) StoreOwners(owners []model.Owner) {
	if len(owners) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var models []mongo.WriteModel
	for _, owner := range owners {
		owner.Type = o.ownerType(owner)
		filter := bson.D{{"_id", owner.ID()}}
		update := bson.D{{"$set", owner}}
		models = append(models, mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update).SetUpsert(true))
	}
	res, err := o.OwnerModel.Model.Collection().BulkWrite(ctx, models)
	if err != nil {
		log.Fatalln(err.Error())
	}
	if res.ModifiedCount > 0 {
		logger.Success(fmt.Sprintf("Updated %d owners!", res.ModifiedCount))
	}
	if res.UpsertedCount > 0 {
		logger.Success(fmt.Sprintf("Inserted %d owners!", res.UpsertedCount))
	}
}

func (o *OwnerHandler) Update() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cursor := database.All(ctx, o.OwnerModel.Name())
	defer database.CloseCursor(ctx, cursor)

	if cursor.RemainingBatchLength() == 0 {
		return nil
	}
	logger.Info("Updating user gists...")
	gistsQuery := model.NewOwnerGistsQuery()
	logger.Info("Updating owner repositories...")
	repositoriesQuery := model.NewOwnerRepositoriesQuery()
	for cursor.Next(ctx) {
		owner := model.Owner{}
		if err := cursor.Decode(&owner); err != nil {
			log.Fatalln(err.Error())
		}

		if o.ownerType(owner) == typeUser {
			var gists []model.Gist
			gistsQuery.OwnerArguments.Login = strconv.Quote(owner.ID())
			if err := o.FetchGists(gistsQuery, &gists); err != nil {
				return err
			}
			o.UpdateGists(owner, gists)
		}

		var repositories []model.Repository
		repositoriesQuery.Field = o.ownerType(owner)
		repositoriesQuery.OwnerArguments.Login = strconv.Quote(owner.ID())
		if err := o.FetchRepositories(repositoriesQuery, &repositories); err != nil {
			return err
		}
		o.UpdateRepositories(owner, repositories)
	}

	return nil
}

func (o *OwnerHandler) FetchGists(q *model.Query, gists *[]model.Gist) error {
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

func (o *OwnerHandler) UpdateGists(owner model.Owner, gists []model.Gist) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.D{{"_id", owner.ID()}}
	update := bson.D{{"$set", bson.D{{"gists", gists}}}}
	if _, err := o.OwnerModel.Model.Collection().UpdateOne(ctx, filter, update); err != nil {
		log.Fatalln(err.Error())
	}
	logger.Success(fmt.Sprintf("Updated %d user gists!", len(gists)))
}

func (o *OwnerHandler) FetchRepositories(q *model.Query, repositories *[]model.Repository) error {
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

func (o *OwnerHandler) UpdateRepositories(owner model.Owner, repositories []model.Repository) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.D{{"_id", owner.ID()}}
	update := bson.D{{"$set", bson.D{{"repositories", repositories}}}}
	if _, err := o.OwnerModel.Model.Collection().UpdateOne(ctx, filter, update); err != nil {
		log.Fatalln(err.Error())
	}
	logger.Success(fmt.Sprintf("Updated %d %s repositories!", len(repositories), owner.Type))
}

func (o *OwnerHandler) Rank() {
	logger.Info("Executing owner rank pipelines...")
	pipelines := []model.RankPipeline{
		o.rankPipeline(typeUser, "followers"),
		o.rankPipeline(typeUser, "gists.forks"),
		o.rankPipeline(typeUser, "gists.stargazers"),
		o.rankPipeline(typeUser, "repositories.forks"),
		o.rankPipeline(typeUser, "repositories.stargazers"),
		o.rankPipeline(typeUser, "repositories.watchers"),
		o.rankPipeline(typeOrganization, "repositories.forks"),
		o.rankPipeline(typeOrganization, "repositories.stargazers"),
		o.rankPipeline(typeOrganization, "repositories.watchers"),
	}
	pipelines = append(pipelines, o.repositoryRankPipelinesByLanguage(typeUser, "forks")...)
	pipelines = append(pipelines, o.repositoryRankPipelinesByLanguage(typeUser, "stargazers")...)
	pipelines = append(pipelines, o.repositoryRankPipelinesByLanguage(typeUser, "watchers")...)
	pipelines = append(pipelines, o.repositoryRankPipelinesByLanguage(typeOrganization, "forks")...)
	pipelines = append(pipelines, o.repositoryRankPipelinesByLanguage(typeOrganization, "stargazers")...)
	pipelines = append(pipelines, o.repositoryRankPipelinesByLanguage(typeOrganization, "watchers")...)

	wg := sync.WaitGroup{}
	batch := o.BatchModel.Get(o.OwnerModel.Name()).Batch
	for _, pipeline := range pipelines {
		wg.Add(1)
		go o.PushRanks(&wg, batch+1, pipeline)
	}
	wg.Wait()
	logger.Success(fmt.Sprintf("Executed %d owner rank pipelines!", len(pipelines)))

	o.BatchModel.Update(o.OwnerModel.Name())
	o.PullRanks(batch)
}

func (o *OwnerHandler) PushRanks(wg *sync.WaitGroup, batch int, pipeline model.RankPipeline) int {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cursor := database.Aggregate(ctx, o.OwnerModel.Name(), pipeline.Pipeline)
	defer database.CloseCursor(ctx, cursor)

	var models []mongo.WriteModel
	count := 0
	for ; cursor.Next(ctx); count++ {
		record := struct {
			ID         string `bson:"_id"`
			TotalCount int    `bson:"total_count"`
		}{}
		if err := cursor.Decode(&record); err != nil {
			log.Fatalln(err.Error())
		}

		rank := model.Rank{
			Rank:       count + 1,
			TotalCount: record.TotalCount,
			Tags:       pipeline.Tags,
			Batch:      batch,
			CreatedAt:  time.Now(),
		}
		filter := bson.D{{"_id", record.ID}}
		update := bson.D{{"$push", bson.D{{"ranks", rank}}}}
		models = append(models, mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update))
		if cursor.RemainingBatchLength() == 0 {
			if _, err := o.OwnerModel.Model.Collection().BulkWrite(ctx, models); err != nil {
				log.Fatalln(err.Error())
			}
			models = models[:0]
		}
	}
	wg.Done()

	return count
}

func (o *OwnerHandler) PullRanks(batch int) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cursor := database.All(ctx, o.OwnerModel.Name())
	defer database.CloseCursor(ctx, cursor)

	var models []mongo.WriteModel
	for cursor.Next(ctx) {
		owner := model.Owner{}
		if err := cursor.Decode(&owner); err != nil {
			log.Fatalln(err.Error())
		}

		filter := bson.D{{"_id", owner.ID()}}
		update := bson.D{{"$pull", bson.D{{"ranks", bson.D{{"batch", bson.D{{"$lte", batch}}}}}}}}
		models = append(models, mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update))
		if cursor.RemainingBatchLength() == 0 {
			if _, err := o.OwnerModel.Model.Collection().BulkWrite(ctx, models); err != nil {
				log.Fatalln(err.Error())
			}
			models = models[:0]
		}
	}
}

func (o *OwnerHandler) CreateIndexes() {
	database.CreateIndexes(o.OwnerModel.Model.Name(), []string{
		"created_at",
		"name",
		"ranks.tags",
	})
}

func (o *OwnerHandler) fetch(q model.Query, res *model.OwnerResponse) (err error) {
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

func (o *OwnerHandler) searchQuery(from time.Time) model.SearchQuery {
	return model.SearchQuery{
		Created: fmt.Sprintf("%s..%s", from.Format(time.RFC3339), from.AddDate(0, 0, 7).Format(time.RFC3339)),
		Repos:   ">=5",
		Sort:    "joined-asc",
	}
}

func (o *OwnerHandler) rankPipeline(ownerType string, object string) model.RankPipeline {
	tags := []string{ownerType}
	tags = append(tags, strings.Split(object, ".")...)

	return model.RankPipeline{
		Pipeline: mongo.Pipeline{
			bson.D{
				{"$match", bson.D{
					{"type", ownerType},
				}},
			},
			bson.D{
				{"$project", bson.D{
					{"_id", "$_id"},
					{"total_count", bson.D{
						{"$sum", fmt.Sprintf("$%s.total_count", object)},
					}},
				}},
			},
			bson.D{
				{"$sort", bson.D{
					{"total_count", -1},
				}},
			},
		},
		Tags: tags,
	}
}

func (o *OwnerHandler) repositoryRankPipelinesByLanguage(ownerType string, object string) (pipelines []model.RankPipeline) {
	for _, language := range util.Languages() {
		pipelines = append(pipelines, model.RankPipeline{
			Pipeline: mongo.Pipeline{
				bson.D{
					{"$match", bson.D{
						{"type", ownerType},
					}},
				},
				bson.D{
					{"$unwind", "$repositories"},
				},
				bson.D{
					{"$match", bson.D{
						{"repositories.primary_language.name", language},
					}},
				},
				bson.D{
					{"$group", bson.D{
						{"_id", "$_id"},
						{"total_count", bson.D{
							{"$sum", fmt.Sprintf("$repositories.%s.total_count", object)},
						}},
					}},
				},
				bson.D{
					{"$sort", bson.D{
						{"total_count", -1},
					}},
				},
			},
			Tags: []string{ownerType, "repository", object, language},
		})
	}
	return
}

func (o *OwnerHandler) ownerType(owner model.Owner) (ownerType string) {
	ownerType = typeUser
	if owner.Followers == nil {
		ownerType = typeOrganization
	}
	return
}
