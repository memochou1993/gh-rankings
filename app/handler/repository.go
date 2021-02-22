package handler

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/memochou1993/gh-rankings/app"
	"github.com/memochou1993/gh-rankings/app/handler/request"
	"github.com/memochou1993/gh-rankings/app/model"
	"net/http"
)

var (
	repositoryModel = model.NewRepositoryModel()
)

func ListRepositories(w http.ResponseWriter, r *http.Request) {
	defer app.CloseBody(r.Body)

	req, err := request.NewRepositoryRequest(r)
	if err != nil {
		response(w, http.StatusUnprocessableEntity, Payload{Error: err.Error()})
		return
	}

	repositories := repositoryModel.List(req)

	response(w, http.StatusOK, Payload{Data: repositories})
}

func ShowRepository(w http.ResponseWriter, r *http.Request) {
	defer app.CloseBody(r.Body)

	owner := mux.Vars(r)["owner"]
	name := mux.Vars(r)["name"]

	repository := repositoryModel.FindByName(fmt.Sprintf("%s/%s", owner, name))
	if repository.ID() == "" {
		response(w, http.StatusNotFound, Payload{Data: nil})
		return
	}

	response(w, http.StatusOK, Payload{Data: repository})
}
