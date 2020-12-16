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

type UserCollection struct {
	Collection
}

type UserResponse struct {
	Data struct {
		Search struct {
			Edges []struct {
				Cursor string `json:"cursor"`
				Node   User   `json:"node"`
			} `json:"edges"`
			PageInfo PageInfo `json:"pageInfo"`
		} `json:"search"`
		User struct {
			AvatarURL    string    `json:"avatarUrl"`
			CreatedAt    time.Time `json:"createdAt"`
			Followers    Directory `json:"followers"`
			Location     string    `json:"location"`
			Login        string    `json:"login"`
			Name         string    `json:"name"`
			Repositories struct {
				TotalCount int `json:"totalCount"`
				Edges      []struct {
					Cursor string     `json:"cursor"`
					Node   Repository `json:"node"`
				} `json:"edges"`
				PageInfo PageInfo `json:"pageInfo"`
			} `json:"repositories"`
		} `json:"user"`
		RateLimit RateLimit `json:"rateLimit"`
	} `json:"data"`
	Errors []Error `json:"errors"`
}

type User struct {
	AvatarURL    string       `json:"avatarUrl" bson:"avatar_url"`
	CreatedAt    time.Time    `json:"createdAt" bson:"created_at"`
	Followers    Directory    `json:"followers" bson:"followers"`
	Location     string       `json:"location" bson:"location"`
	Login        string       `json:"login" bson:"_id"`
	Name         string       `json:"name" bson:"name"`
	Repositories []Repository `json:"repositories" bson:"repositories,omitempty"`
	Ranks        *Ranks       `json:"ranks" bson:"ranks,omitempty"`
}

type Repository struct {
	Name            string `json:"name" bson:"name"`
	PrimaryLanguage struct {
		Name string `json:"name" bson:"name"`
	} `json:"primaryLanguage" bson:"primary_language"`
	Stargazers Directory `json:"stargazers" bson:"stargazers"`
}

type Ranks struct {
	RepositoryStars *RepositoryStars `bson:"repository_stars,omitempty"`
}

type RepositoryStars struct {
	Rank       int       `bson:"rank"`
	TotalCount int       `bson:"total_count"`
	CreatedAt  time.Time `bson:"created_at"`
}

func NewUserCollection() *UserCollection {
	return &UserCollection{
		Collection: Collection{
			name: "users",
		},
	}
}

func (u *UserCollection) Init(starter chan<- struct{}) {
	logger.Info("Initializing user collection...")
	u.CreateIndexes()
	logger.Success("User collection initialized!")
	starter <- struct{}{}
}

func (u *UserCollection) Collect() error {
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

func (u *UserCollection) Travel(from *time.Time, q *Query) error {
	to := time.Now()
	if from.After(to) {
		return nil
	}

	q.SearchArguments.Query = strconv.Quote(util.JoinStruct(SearchQuery{
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

func (u *UserCollection) FetchUsers(q *Query, users *[]User) error {
	res := UserResponse{}
	if err := u.Fetch(q, &res); err != nil {
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

func (u *UserCollection) StoreUsers(users []User) {
	if len(users) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var models []mongo.WriteModel
	for _, user := range users {
		filter := bson.D{{"_id", user.Login}}
		model := bson.D{{"$set", user}}
		models = append(models, mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(model).SetUpsert(true))
	}
	res, err := database.GetCollection("users").BulkWrite(ctx, models)
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

func (u *UserCollection) Update() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts := options.Find().SetBatchSize(1000)
	cursor, err := u.GetCollection().Find(ctx, bson.D{}, opts)
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
	logger.Info("Updating user repositories...")

	q := Query{
		Schema: ReadQuery("user_repositories"),
		RepositoriesArguments: RepositoriesArguments{
			First:             100,
			OrderBy:           "{field:STARGAZERS,direction:DESC}",
			OwnerAffiliations: "OWNER",
		},
	}
	for cursor.Next(ctx) {
		user := User{}
		if err := cursor.Decode(&user); err != nil {
			log.Fatalln(err.Error())
		}
		q.UserArguments.Login = strconv.Quote(user.Login)

		var repos []Repository
		if err := u.FetchRepositories(&q, &repos); err != nil {
			return err
		}
		u.UpdateRepositories(user, repos)
	}

	return nil
}

func (u *UserCollection) FetchRepositories(q *Query, repos *[]Repository) error {
	res := UserResponse{}
	if err := u.Fetch(q, &res); err != nil {
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

func (u *UserCollection) UpdateRepositories(user User, repos []Repository) {
	if len(repos) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.D{{"_id", user.Login}}
	update := bson.D{{"$set", bson.D{{"repositories", repos}}}}
	u.GetCollection().FindOneAndUpdate(ctx, filter, update)
	logger.Success(fmt.Sprintf("Updated %d user repositories!", len(repos)))
}

func (u *UserCollection) RankRepositoryStars() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pipeline := mongo.Pipeline{
		bson.D{
			{"$project", bson.D{
				{"login", "$login"},
				{"ranks", bson.D{
					{"repository_stars", bson.D{
						{"total_count", bson.D{
							{"$sum", "$repositories.stargazers.total_count"},
						}},
					}},
				}},
			}},
		},
		bson.D{
			{"$sort", bson.D{
				{"ranks.repository_stars.total_count", -1},
			}},
		},
	}
	opts := options.Aggregate().SetBatchSize(1000)
	cursor, err := u.GetCollection().Aggregate(ctx, pipeline, opts)
	if err != nil {
		log.Fatalln(err.Error())
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Fatalln(err.Error())
		}
	}()

	if cursor.RemainingBatchLength() == 0 {
		return
	}
	logger.Info("Ranking user repository stars...")

	var models []mongo.WriteModel
	count := 0
	for ; cursor.Next(ctx); count++ {
		user := User{
			Ranks: &Ranks{
				RepositoryStars: &RepositoryStars{
					Rank:      count + 1,
					CreatedAt: time.Now(),
				},
			},
		}
		if err := cursor.Decode(&user); err != nil {
			log.Fatalln(err.Error())
		}

		filter := bson.D{{"_id", user.Login}}
		model := bson.D{{"$set", bson.D{{"ranks.repository_stars", user.Ranks.RepositoryStars}}}}
		models = append(models, mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(model))
		if cursor.RemainingBatchLength() == 0 {
			_, err := database.GetCollection("users").BulkWrite(ctx, models)
			if err != nil {
				log.Fatalln(err.Error())
			}
			models = models[:0]
		}
	}
	logger.Success(fmt.Sprintf("Ranked %d user repository stars!", count))
}

func (u *UserCollection) Fetch(q *Query, res *UserResponse) (err error) {
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

func (u *UserCollection) GetByLogin(login string) (user User) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := u.GetCollection().FindOne(ctx, bson.D{{"_id", login}}).Decode(&user); err != nil {
		log.Fatalln(err.Error())
	}

	return user
}

func (u *UserCollection) CreateIndexes() {
	if len(database.GetIndexes(u.name)) > 0 {
		return
	}

	database.CreateIndexes(u.name, []string{
		"created_at",
		"ranks.repository_stars.rank",
	})
}
