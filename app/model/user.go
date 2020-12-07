package model

import (
	"context"
	"fmt"
	"github.com/memochou1993/github-rankings/app"
	"github.com/memochou1993/github-rankings/app/database"
	"github.com/memochou1993/github-rankings/app/query"
	"go.mongodb.org/mongo-driver/bson"
	"log"
	"time"
)

const (
	CollectionUsers = "users"
	SearchUsers     = "search_users"
)

type Users struct {
	Data struct {
		Search struct {
			UserCount int `json:"userCount"`
			Edges     []struct {
				Cursor string `json:"cursor"`
				Node   User   `json:"node"`
			} `json:"edges"`
			PageInfo query.PageInfo `json:"pageInfo"`
		} `json:"search"`
		RateLimit query.RateLimit `json:"rateLimit"`
	} `json:"data"`
}

type User struct {
	Login string `json:"login"`
	Name  string `json:"name"`
}

func (u *Users) Init() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	count, err := database.Count(ctx, CollectionUsers)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	if err := u.Collect(ctx); err != nil {
		return err
	}
	if err := u.Index(ctx); err != nil {
		return err
	}

	return nil
}

func (u *Users) Collect(ctx context.Context) error {
	layout := "2006-01-02"
	date := time.Date(2007, time.October, 1, 0, 0, 0, 0, time.UTC)
	for ; date.Before(time.Now()); date.AddDate(0, 0, 7) {
		created := fmt.Sprintf("%s..%s", date.Format(layout), date.AddDate(0, 0, 6).Format(layout))
		followers := ">=10"
		repos := ">=5"
		args := &query.SearchArguments{
			First: 100,
			Query: fmt.Sprintf("\"created:%s followers:%s repos:%s\"", created, followers, repos),
			Type:  "USER",
		}
		for {
			if u.Data.RateLimit.ResetAt != "" {
				resetAt, err := time.Parse(time.RFC3339, u.Data.RateLimit.ResetAt)
				if err != nil {
					log.Fatal(err.Error())
				}
				if u.Data.RateLimit.Remaining == 0 {
					duration := resetAt.Sub(time.Now().UTC())
					log.Println(fmt.Sprintf("Wait about %d minutes for next call", int(duration.Minutes())))
					time.Sleep(duration)
				}
			}
			if u.Data.Search.PageInfo.EndCursor != "" {
				args.After = fmt.Sprintf("\"%s\"", u.Data.Search.PageInfo.EndCursor)
			}
			log.Println(fmt.Sprintf("Searching users with arguments: \"%s\"", query.Join(args)))
			if err := u.Search(ctx, args); err != nil {
				return err
			}
			log.Println(fmt.Sprintf("The call's rate limit status: %d points remaining", u.Data.RateLimit.Remaining))
			if len(u.Data.Search.Edges) == 0 {
				break
			}
			if err := u.Store(ctx); err != nil {
				return err
			}
			log.Println(fmt.Sprintf("Discovered %d users", len(u.Data.Search.Edges)))
			if !u.Data.Search.PageInfo.HasNextPage {
				u.Data.Search.PageInfo.EndCursor = ""
				break
			}
		}
		date = date.AddDate(0, 0, 7)
	}

	return nil
}

func (u *Users) Search(ctx context.Context, args *query.SearchArguments) error {
	return app.Fetch(ctx, []byte(args.Read(SearchUsers)), u)
}

func (u *Users) Store(ctx context.Context) error {
	var documents []interface{}
	for _, edge := range u.Data.Search.Edges {
		documents = append(documents, bson.M{
			"login": edge.Node.Login,
			"name":  edge.Node.Name,
		})
	}

	_, err := database.GetCollection(CollectionUsers).InsertMany(ctx, documents)

	return err
}

func (u *Users) Index(ctx context.Context) error {
	return database.CreateIndexes(ctx, CollectionUsers, []string{"name"})
}
