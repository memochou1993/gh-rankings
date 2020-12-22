package model

import "time"

type Repository struct {
	CreatedAt         time.Time `json:"createdAt" bson:"created_at"`
	Forks             Directory `json:"forks" bson:"forks"`
	Name              string    `json:"name" bson:"name"`
	NameWithOwner     string    `json:"nameWithOwner" bson:"_id"`
	OpenGraphImageUrl string    `json:"openGraphImageUrl" bson:"open_graph_image_url"`
	Owner             struct {
		Login string `json:"login" bson:"login"`
	} `json:"owner" bson:"owner"`
	PrimaryLanguage struct {
		Name string `json:"name" bson:"name"`
	} `json:"primaryLanguage" bson:"primary_language"`
	Stargazers Directory `json:"stargazers" bson:"stargazers"`
	Watchers   Directory `json:"watchers" bson:"watchers"`
}

func (r Repository) ID() string {
	return r.NameWithOwner
}

type RepositoryResponse struct {
	Data struct {
		Search struct {
			Edges []struct {
				Cursor string     `json:"cursor"`
				Node   Repository `json:"node"`
			} `json:"edges"`
			PageInfo `json:"pageInfo"`
		} `json:"search"`
		RateLimit `json:"rateLimit"`
	} `json:"data"`
	Errors []Error `json:"errors"`
}

type RepositoryModel struct {
	*Model
}

func NewRepositoryModel() *RepositoryModel {
	return &RepositoryModel{
		&Model{
			name: "repositories",
		},
	}
}
