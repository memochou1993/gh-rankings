package app

import (
	"context"
	"fmt"
	"github.com/memochou1993/github-rankings/database"
	"github.com/memochou1993/github-rankings/logger"
	"github.com/memochou1993/github-rankings/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"strconv"
	"time"
)

type UserModel struct {
	Model
}

type UserResponse struct {
	Data struct {
		Search struct {
			Edges []struct {
				Cursor string `json:"cursor"`
				Node   User   `json:"node"`
			} `json:"edges"`
			PageInfo `json:"pageInfo"`
		} `json:"search"`
		User struct {
			AvatarURL string    `json:"avatarUrl"`
			CreatedAt time.Time `json:"createdAt"`
			Followers Directory `json:"followers"`
			Gists     struct {
				Edges []struct {
					Cursor string `json:"cursor"`
					Node   Gist   `json:"node"`
				} `json:"edges"`
				PageInfo   PageInfo `json:"pageInfo"`
				TotalCount int      `json:"totalCount"`
			} `json:"gists"`
			Location     string `json:"location"`
			Login        string `json:"login"`
			Name         string `json:"name"`
			Repositories struct {
				Edges []struct {
					Cursor string     `json:"cursor"`
					Node   Repository `json:"node"`
				} `json:"edges"`
				PageInfo   PageInfo `json:"pageInfo"`
				TotalCount int      `json:"totalCount"`
			} `json:"repositories"`
		} `json:"user"`
		RateLimit `json:"rateLimit"`
	} `json:"data"`
	Errors []Error `json:"errors"`
}

type UserRank struct {
	Login string `bson:"_id"`
	Rank
}

type User struct {
	AvatarURL    string       `json:"avatarUrl" bson:"avatar_url"`
	CreatedAt    time.Time    `json:"createdAt" bson:"created_at"`
	Followers    Directory    `json:"followers" bson:"followers"`
	Location     string       `json:"location" bson:"location"`
	Login        string       `json:"login" bson:"_id"`
	Name         string       `json:"name" bson:"name"`
	Repositories []Repository `json:"repositories" bson:"repositories,omitempty"`
	Ranks        *struct {
		GistStars       *Rank `bson:"gist_stars,omitempty"`
		RepositoryStars *Rank `bson:"repository_stars,omitempty"`
	} `bson:"ranks,omitempty"`
}

type Gist struct {
	Name       string    `json:"name" bson:"name"`
	Stargazers Directory `json:"stargazers" bson:"stargazers"`
}

func NewUserModel() *UserModel {
	return &UserModel{
		Model{
			name: "users",
		},
	}
}

func (u *UserModel) Init(starter chan<- struct{}) {
	logger.Info("Initializing user collection...")
	u.CreateIndexes()
	logger.Success("User collection initialized!")
	starter <- struct{}{}
}

func (u *UserModel) Collect() error {
	logger.Info("Collecting users...")
	from := time.Date(2007, time.October, 1, 0, 0, 0, 0, time.UTC)
	q := Query{
		Schema: ReadQuery("users"),
		SearchArguments: SearchArguments{
			First: 100,
			Type:  "USER",
		},
	}

	return u.Travel(&from, &q)
}

func (u *UserModel) Travel(from *time.Time, q *Query) error {
	to := time.Now()
	if from.After(to) {
		return nil
	}

	q.SearchArguments.Query = strconv.Quote(util.ParseStruct(SearchQuery{
		Created:   fmt.Sprintf("%s..%s", from.Format(time.RFC3339), from.AddDate(0, 0, 7).Format(time.RFC3339)),
		Followers: ">=10",
		Repos:     ">=5",
		Sort:      "joined",
	}, " "))

	var users []User
	if err := u.FetchUsers(q, &users); err != nil {
		return err
	}
	u.StoreUsers(users)
	*from = from.AddDate(0, 0, 7)

	return u.Travel(from, q)
}

func (u *UserModel) FetchUsers(q *Query, users *[]User) error {
	res := UserResponse{}
	if err := u.Fetch(*q, &res); err != nil {
		return err
	}
	for _, edge := range res.Data.Search.Edges {
		*users = append(*users, edge.Node)
	}
	res.Data.RateLimit.Break()
	if !res.Data.Search.PageInfo.HasNextPage {
		q.SearchArguments.After = ""
		return nil
	}
	q.SearchArguments.After = strconv.Quote(res.Data.Search.PageInfo.EndCursor)

	return u.FetchUsers(q, users)
}

func (u *UserModel) StoreUsers(users []User) {
	if len(users) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var models []mongo.WriteModel
	for _, user := range users {
		filter := bson.D{{"_id", user.Login}}
		update := bson.D{{"$set", user}}
		model := mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update).SetUpsert(true)
		models = append(models, model)
	}
	res, err := u.Collection().BulkWrite(ctx, models)
	if err != nil {
		log.Fatalln(err.Error())
	}
	if res.ModifiedCount > 0 {
		logger.Success(fmt.Sprintf("Updated %d users!", res.ModifiedCount))
	}
	if res.UpsertedCount > 0 {
		logger.Success(fmt.Sprintf("Inserted %d users!", res.UpsertedCount))
	}
}

func (u *UserModel) Update() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts := options.Find().SetBatchSize(1000)
	cursor, err := u.Collection().Find(ctx, bson.D{}, opts)
	if err != nil {
		log.Fatalln(err.Error())
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Fatalln(err.Error())
		}
	}()

	if cursor.RemainingBatchLength() == 0 {
		return nil
	}
	logger.Info("Updating user gists...")
	gistsQuery := NewGistsQuery()
	logger.Info("Updating user repositories...")
	reposQuery := NewReposQuery()
	for cursor.Next(ctx) {
		user := User{}
		if err := cursor.Decode(&user); err != nil {
			log.Fatalln(err.Error())
		}

		var gists []Gist
		gistsQuery.UserArguments.Login = strconv.Quote(user.Login)
		if err := u.FetchGists(gistsQuery, &gists); err != nil {
			return err
		}
		u.UpdateGists(user, gists)

		var repos []Repository
		reposQuery.UserArguments.Login = strconv.Quote(user.Login)
		if err := u.FetchRepositories(reposQuery, &repos); err != nil {
			return err
		}
		u.UpdateRepositories(user, repos)
	}

	return nil
}

func (u *UserModel) FetchGists(q *Query, gists *[]Gist) error {
	res := UserResponse{}
	if err := u.Fetch(*q, &res); err != nil {
		return err
	}
	for _, edge := range res.Data.User.Gists.Edges {
		*gists = append(*gists, edge.Node)
	}
	res.Data.RateLimit.Break()
	if !res.Data.User.Gists.PageInfo.HasNextPage {
		q.GistsArguments.After = ""
		return nil
	}
	q.GistsArguments.After = strconv.Quote(res.Data.User.Gists.PageInfo.EndCursor)

	return u.FetchGists(q, gists)
}

func (u *UserModel) UpdateGists(user User, gists []Gist) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.D{{"_id", user.Login}}
	update := bson.D{{"$set", bson.D{{"gists", gists}}}}
	u.Collection().FindOneAndUpdate(ctx, filter, update)
	logger.Success(fmt.Sprintf("Updated %d user gists!", len(gists)))
}

func (u *UserModel) FetchRepositories(q *Query, repos *[]Repository) error {
	res := UserResponse{}
	if err := u.Fetch(*q, &res); err != nil {
		return err
	}
	for _, edge := range res.Data.User.Repositories.Edges {
		*repos = append(*repos, edge.Node)
	}
	res.Data.RateLimit.Break()
	if !res.Data.User.Repositories.PageInfo.HasNextPage {
		q.RepositoriesArguments.After = ""
		return nil
	}
	q.RepositoriesArguments.After = strconv.Quote(res.Data.User.Repositories.PageInfo.EndCursor)

	return u.FetchRepositories(q, repos)
}

func (u *UserModel) UpdateRepositories(user User, repos []Repository) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.D{{"_id", user.Login}}
	update := bson.D{{"$set", bson.D{{"repositories", repos}}}}
	u.Collection().FindOneAndUpdate(ctx, filter, update)
	logger.Success(fmt.Sprintf("Updated %d user repositories!", len(repos)))
}

func (u *UserModel) RankFollowers() {
	logger.Info("Ranking user followers...")
	pipeline := mongo.Pipeline{
		bson.D{
			{"$project", bson.D{
				{"login", "$login"},
				{"rank", bson.D{
					{"total_count", bson.D{
						{"$sum", "$followers.total_count"},
					}},
				}},
			}},
		},
		bson.D{
			{"$sort", bson.D{
				{"rank.total_count", -1},
			}},
		},
	}
	field := "ranks.followers"
	count := u.Rank(pipeline, field)
	logger.Success(fmt.Sprintf("Ranked %d user followers!", count))
}

func (u *UserModel) RankGistStars() {
	logger.Info("Ranking user gist stars...")
	pipeline := mongo.Pipeline{
		bson.D{
			{"$project", bson.D{
				{"login", "$login"},
				{"rank", bson.D{
					{"total_count", bson.D{
						{"$sum", "$gists.stargazers.total_count"},
					}},
				}},
			}},
		},
		bson.D{
			{"$sort", bson.D{
				{"rank.total_count", -1},
			}},
		},
	}
	field := "ranks.gist_stars"
	count := u.Rank(pipeline, field)
	logger.Success(fmt.Sprintf("Ranked %d user gist stars!", count))
}

func (u *UserModel) RankRepositoryStars() {
	logger.Info("Ranking user repository stars...")
	pipeline := mongo.Pipeline{
		bson.D{
			{"$project", bson.D{
				{"login", "$login"},
				{"rank", bson.D{
					{"total_count", bson.D{
						{"$sum", "$repositories.stargazers.total_count"},
					}},
				}},
			}},
		},
		bson.D{
			{"$sort", bson.D{
				{"rank.total_count", -1},
			}},
		},
	}
	field := "ranks.repository_stars"
	count := u.Rank(pipeline, field)
	logger.Success(fmt.Sprintf("Ranked %d user repository stars!", count))
}

func (u *UserModel) Rank(pipeline []bson.D, field string) int {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts := options.Aggregate().SetBatchSize(1000)
	cursor, err := u.Collection().Aggregate(ctx, pipeline, opts)
	if err != nil {
		log.Fatalln(err.Error())
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Fatalln(err.Error())
		}
	}()

	var models []mongo.WriteModel
	count := 0
	for ; cursor.Next(ctx); count++ {
		rank := UserRank{
			Rank: Rank{
				Rank:      count + 1,
				CreatedAt: time.Now(),
			},
		}
		if err := cursor.Decode(&rank); err != nil {
			log.Fatalln(err.Error())
		}

		filter := bson.D{{"_id", rank.Login}}
		update := bson.D{{"$set", bson.D{{field, rank.Rank}}}}
		model := mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update)
		models = append(models, model)
		if cursor.RemainingBatchLength() == 0 {
			_, err := u.Collection().BulkWrite(ctx, models)
			if err != nil {
				log.Fatalln(err.Error())
			}
			models = models[:0]
		}
	}

	return count
}

func (u *UserModel) Fetch(q Query, res *UserResponse) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := Fetch(ctx, q, res); err != nil {
		return err
	}
	for _, err := range res.Errors {
		return err
	}

	return nil
}

func (u *UserModel) GetByLogin(login string) (user User) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.D{{"_id", login}}
	if err := u.Collection().FindOne(ctx, filter).Decode(&user); err != nil {
		logger.Warning(err.Error())
	}

	return user
}

func (u *UserModel) CreateIndexes() {
	if len(database.Indexes(u.name)) > 0 {
		return
	}

	database.CreateIndexes(u.name, []string{
		"created_at",
		"name",
		"ranks.followers.rank",
		"ranks.gist_stars.rank",
		"ranks.repository_stars.rank",
	})
}
