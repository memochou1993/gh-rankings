package handler

import (
	"context"
	"github.com/memochou1993/github-rankings/app/model"
	"github.com/memochou1993/github-rankings/app/worker"
	"log"
	"net/http"
	"strconv"
	"strings"
)

func ListRepositories(w http.ResponseWriter, r *http.Request) {
	defer closeBody(r)

	nameWithOwner := r.URL.Query().Get("nameWithOwner")
	tags := strings.Split(r.URL.Query().Get("tags"), ",")
	timestamp := worker.RepositoryWorker.Timestamp
	page, err := strconv.ParseInt(r.URL.Query().Get("page"), 10, 64)
	if page < 0 || err != nil {
		page = 1
	}

	var repositories []model.RepositoryRank
	if timestamp == nil {
		response(w, http.StatusOK, repositories)
		return
	}
	args := model.RepositoryRankArguments{
		NameWithOwner: nameWithOwner,
		Tags:          tags,
		CreatedAt:     *timestamp,
		Page:          int(page),
	}
	cursor := model.NewRepositoryRankModel().List(args)
	if err := cursor.All(context.Background(), &repositories); err != nil {
		log.Fatalln(err.Error())
	}

	response(w, http.StatusOK, repositories)
}
