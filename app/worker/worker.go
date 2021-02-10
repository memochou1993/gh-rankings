package worker

import (
	"github.com/memochou1993/gh-rankings/app/model"
	"github.com/memochou1993/gh-rankings/logger"
	"github.com/spf13/viper"
	"log"
	"time"
)

const (
	timestampOwnerRanks        = "TIMESTAMP_OWNER_RANKS"
	timestampOrganizationRanks = "TIMESTAMP_ORGANIZATION_RANKS"
	timestampRepositoryRanks   = "TIMESTAMP_REPOSITORY_RANKS"
)

var (
	RankModel = model.NewRankModel()
)

var (
	OwnerWorker        *ownerWorker
	OrganizationWorker *organizationWorker
	RepositoryWorker   *repositoryWorker
)

type Interface interface {
	Init()
	Collect() error
	Rank()
}

type Worker struct {
	Timestamp time.Time
}

func (w *Worker) loadTimestamp(key string) {
	w.Timestamp = time.Unix(0, viper.GetInt64(key))
	if w.Timestamp == time.Unix(0, 0) {
		w.Timestamp = time.Now()
	}
}

func (w *Worker) saveTimestamp(key string, t time.Time) {
	w.Timestamp = t
	viper.Set(key, t.UnixNano())
	if err := viper.WriteConfig(); err != nil {
		log.Fatal(err.Error())
	}
}

func Init() {
	RankModel.CreateIndexes()

	// FIXME: should refactor
	// OwnerWorker = NewOwnerWorker()
	// go Run(OwnerWorker)

	OrganizationWorker = NewOrganizationWorker()
	go Run(OrganizationWorker)

	RepositoryWorker = NewRepositoryWorker()
	go Run(RepositoryWorker)
}

func Run(worker Interface) {
	worker.Init()
	t := time.NewTicker(7 * 24 * time.Hour)
	for ; true; <-t.C {
		if err := worker.Collect(); err != nil {
			logger.Error(err.Error())
		}
		worker.Rank()
	}
}

func NewWorker() *Worker {
	return &Worker{}
}
