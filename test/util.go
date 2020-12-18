package test

import (
	"context"
	"github.com/memochou1993/github-rankings/app/model"
	"github.com/memochou1993/github-rankings/database"
	"log"
	"os"
	"path"
	"runtime"
)

func ChangeDirectory() {
	_, file, _, _ := runtime.Caller(0)
	dir := path.Join(path.Dir(file), "..")
	if err := os.Chdir(dir); err != nil {
		log.Fatalln(err.Error())
	}
}

func DropCollection(c model.Interface) {
	if err := c.Collection().Drop(context.Background()); err != nil {
		log.Fatalln(err.Error())
	}
}

func DropDatabase() {
	if err := database.Database().Drop(context.Background()); err != nil {
		log.Fatalln(err.Error())
	}
}
