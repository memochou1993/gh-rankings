package model

import (
	"encoding/json"
	"github.com/memochou1993/gh-rankings/util"
	"log"
	"strings"
)

type Payload struct {
	Query string `json:"query"`
}

type Query struct {
	Schema string
	Field  string
	SearchArguments
	OwnerArguments
	GistsArguments
	RepositoriesArguments
}

func (q Query) String() string {
	query := q.Schema
	query = strings.Replace(query, "<Field>", q.Field, 1)
	query = strings.Replace(query, "<SearchArguments>", util.ParseStruct(q.SearchArguments, ","), 1)
	query = strings.Replace(query, "<OwnerArguments>", util.ParseStruct(q.OwnerArguments, ","), 1)
	query = strings.Replace(query, "<GistsArguments>", util.ParseStruct(q.GistsArguments, ","), 1)
	query = strings.Replace(query, "<RepositoriesArguments>", util.ParseStruct(q.RepositoriesArguments, ","), 1)
	b, err := json.Marshal(Payload{Query: query})
	if err != nil {
		log.Fatal(err.Error())
	}
	return string(b)
}

type SearchArguments struct {
	After string `json:"after,omitempty"`
	First int    `json:"first,omitempty"`
	Query string `json:"query,omitempty"`
	Type  string `json:"type,omitempty"`
}

type OwnerArguments struct {
	Login string `json:"login,omitempty"`
}

type GistsArguments struct {
	After   string `json:"after,omitempty"`
	First   int    `json:"first,omitempty"`
	OrderBy string `json:"orderBy,omitempty"`
}

type RepositoriesArguments struct {
	After             string `json:"after,omitempty"`
	First             int    `json:"first,omitempty"`
	OrderBy           string `json:"orderBy,omitempty"`
	OwnerAffiliations string `json:"ownerAffiliations,omitempty"`
}

type SearchQuery struct {
	Created   string `json:"created,omitempty"`
	Followers string `json:"followers,omitempty"`
	Fork      string `json:"fork,omitempty"`
	Repos     string `json:"repos,omitempty"`
	Sort      string `json:"sort,omitempty"`
	Stars     string `json:"stars,omitempty"`
	Type      string `json:"type,omitempty"`
}

type Items struct {
	TotalCount int `json:"totalCount,omitempty" bson:"total_count,omitempty"`
}

func NewOwnerQuery() *Query {
	return &Query{
		Schema: util.ReadQuery("owners"),
		SearchArguments: SearchArguments{
			First: 100,
			Type:  "USER",
		},
	}
}

func NewOwnerGistQuery() *Query {
	return &Query{
		Schema: util.ReadQuery("owner_gists"),
		GistsArguments: GistsArguments{
			First:   100,
			OrderBy: "{field:CREATED_AT,direction:ASC}",
		},
	}
}

func NewOwnerRepositoryQuery() *Query {
	return &Query{
		Schema: util.ReadQuery("owner_repositories"),
		RepositoriesArguments: RepositoriesArguments{
			First:             100,
			OrderBy:           "{field:CREATED_AT,direction:ASC}",
			OwnerAffiliations: "OWNER",
		},
	}
}

func NewRepositoryQuery() *Query {
	return &Query{
		Schema: util.ReadQuery("repositories"),
		SearchArguments: SearchArguments{
			First: 100,
			Type:  "REPOSITORY",
		},
	}
}
