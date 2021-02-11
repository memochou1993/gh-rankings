package pipeline

import (
	"fmt"
	"github.com/memochou1993/gh-rankings/app/resource"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type Pipeline struct {
	Pipeline *mongo.Pipeline
	Type     string
	Field    string
	Language string
	Location string
}

func RankByField(rankType string, field string) *Pipeline {
	return &Pipeline{
		Pipeline: &mongo.Pipeline{
			stageProject(field),
			stageSort(),
		},
		Type:  rankType,
		Field: field,
	}
}

func RankByLocation(rankType string, field string) (pipelines []*Pipeline) {
	for _, location := range resource.Locations {
		pipelines = append(pipelines, &Pipeline{
			Pipeline: &mongo.Pipeline{
				stageMatch("parsed_location", location.Name),
				stageProject(field),
				stageSort(),
			},
			Type:     rankType,
			Field:    field,
			Location: location.Name,
		})
		for _, city := range location.Cities {
			location := fmt.Sprintf("%s, %s", city.Name, location.Name)
			pipelines = append(pipelines, &Pipeline{
				Pipeline: &mongo.Pipeline{
					stageMatch("parsed_city", location),
					stageProject(field),
					stageSort(),
				},
				Type:     rankType,
				Field:    field,
				Location: location,
			})
		}
	}
	return
}

func RankOwnerRepositoryByLanguage(rankType string, field string) (pipelines []*Pipeline) {
	for _, language := range resource.Languages {
		pipelines = append(pipelines, &Pipeline{
			Pipeline: &mongo.Pipeline{
				stageUnwind("repositories"),
				stageMatch("repositories.primary_language.name", language.Name),
				stageGroup(field),
				stageSort(),
			},
			Type:     rankType,
			Field:    field,
			Language: language.Name,
		})
	}
	return
}

func RankRepositoryByLanguage(rankType string, field string) (pipelines []*Pipeline) {
	for _, language := range resource.Languages {
		pipelines = append(pipelines, &Pipeline{
			Pipeline: &mongo.Pipeline{
				stageMatch("primary_language.name", language.Name),
				stageProject(field),
				stageSort(),
			},
			Type:     rankType,
			Field:    field,
			Language: language.Name,
		})
	}
	return
}

func stageUnwind(field string) bson.D {
	return bson.D{
		{"$unwind", fmt.Sprintf("$%s", field)},
	}
}

func stageMatch(key string, value string) bson.D {
	return bson.D{
		{"$match", bson.D{
			{key, value},
		}},
	}
}

func stageProject(field string) bson.D {
	return bson.D{
		{"$project", bson.D{
			{"_id", "$_id"},
			{"image_url", "$image_url"},
			{"total_count", bson.D{
				{"$sum", fmt.Sprintf("$%s.total_count", field)},
			}},
		}},
	}
}

func stageGroup(field string) bson.D {
	return bson.D{
		{"$group", bson.D{
			{"_id", "$_id"},
			{"image_url", bson.D{
				{"$first", "$image_url"},
			}},
			{"total_count", bson.D{
				{"$sum", fmt.Sprintf("$%s.total_count", field)},
			}},
		}},
	}
}

func stageSort() bson.D {
	return bson.D{
		{"$sort", bson.D{
			{"total_count", -1},
		}},
	}
}
