package test

import (
	"github.com/memochou1993/github-rankings/app"
	"github.com/memochou1993/github-rankings/database"
	"github.com/memochou1993/github-rankings/logger"
	"github.com/memochou1993/github-rankings/util"
	"go.mongodb.org/mongo-driver/bson"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	setUp()
	code := m.Run()
	tearDown()
	os.Exit(code)
}

func setUp() {
	ChangeDirectory()
	util.LoadEnv()
	database.Init()
	logger.Init()
}

func TestFetchUsers(t *testing.T) {
	u := app.NewUserCollection()

	q := app.Query{
		Schema: app.ReadQuery("users"),
		SearchArguments: app.SearchArguments{
			First: 100,
			Type:  "USER",
		},
	}
	q.SearchArguments.Query = q.String("created:2020-01-01..2020-01-01 followers:>=1 repos:>=10")

	var users []interface{}
	if err := u.FetchUsers(&q, &users); err != nil {
		t.Error(err.Error())
	}
	if len(users) == 0 {
		t.Fail()
	}

	DropCollection(u)
}

func TestStoreUsers(t *testing.T) {
	u := app.NewUserCollection()

	var users []interface{}
	users = append(users, bson.D{})
	if err := u.StoreUsers(users); err != nil {
		t.Error(err.Error())
	}
	if count := database.Count(u.GetName()); count != 1 {
		t.Fail()
	}

	DropCollection(u)
}

func TestIndexUsers(t *testing.T) {
	u := app.NewUserCollection()

	if err := u.Index(); err != nil {
		t.Error(err.Error())
	}

	indexes := database.GetIndexes(u.GetName())
	if len(indexes) == 0 {
		t.Fail()
	}

	DropCollection(u)
}

func tearDown() {
	DropDatabase()
}
