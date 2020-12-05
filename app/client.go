package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/memochou1993/github-rankings/app/model"
	"github.com/memochou1993/github-rankings/util"
	"io"
	"log"
	"net/http"
	"os"
)

type Query struct {
	Query string `json:"query"`
}

var client *http.Client

func init() {
	util.LoadEnv()
	initClient()
}

func initClient() {
	client = http.DefaultClient
}

func SearchInitialUsers(ctx context.Context) (model.InitialUsers, error) {
	users := model.InitialUsers{}
	args := model.SearchArguments{
		First: 100,
		Query: "\"repos:>=5 followers:>=10\"",
		Type:  "USER",
	}

	err := fetch(ctx, []byte(users.GetQuery(args)), &users)

	return users, err
}

func fetch(ctx context.Context, q []byte, v interface{}) error {
	body := &bytes.Buffer{}
	if err := json.NewEncoder(body).Encode(Query{Query: string(q)}); err != nil {
		return err
	}

	resp, err := post(ctx, body)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Println(err.Error())
		}
	}()
	if err != nil {
		return err
	}

	return json.NewDecoder(resp.Body).Decode(&v)
}

func post(ctx context.Context, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, os.Getenv("API_URL"), body)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("API_TOKEN")))

	return client.Do(req.WithContext(ctx))
}
