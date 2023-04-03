package main

import (
	"context"
	"database/sql"
	_ "github.com/lib/pq"
	"github.com/robfig/cron/v3"
	"log"
	"math/rand"
	"os"
	"surelink-go/api/controller"
	"surelink-go/api/routes"
	"surelink-go/api/service"
	_ "surelink-go/api/service"
	"surelink-go/cronjob"
	"surelink-go/infrastructure"
	"surelink-go/util"
	"time"
)

func main() {
	initialTests()

	globalConfig, err := util.LoadGlobalConfig(".")
	if err != nil {
		log.Fatal("can not load global config", err)
	}

	//miscellaneous
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	cronScheduler := cron.New()
	go cronScheduler.Run()

	//database and cache
	conn, err := sql.Open(globalConfig.DBDriver, globalConfig.DBSource)
	if err != nil {
		log.Fatal("can't connect to the database", err)
	}
	store := infrastructure.NewStore(conn)

	cache := infrastructure.NewCache(globalConfig.RedisUrl)

	// initialize gin router
	log.Println("Initializing Routes")
	ginRouter := infrastructure.NewGinRouter()

	//initialize service
	utilityService := service.NewUtilityService(cache, random)

	// captcha
	captchaService := service.NewCaptchaService(cache)
	captchaController := controller.NewCaptchaController(captchaService)
	captchaRoute := routes.NewCaptchaRoute(captchaController, ginRouter)
	captchaRoute.Setup()

	//redirection
	redirectionService := service.NewRedirectionService(store, cache, &utilityService)
	redirectionController := controller.NewRedirectionController(redirectionService)
	redirectionRoute := routes.NewRedirectionRoute(redirectionController, ginRouter)
	redirectionRoute.Setup()

	//cronjobs
	go func() {
		cronJobCtx := context.Background()

		captchaCronJob := cronjob.NewCaptchaCronJob(cache)
		_, errCron := cronScheduler.AddFunc(util.CronSpecEveryOneMin, func() {
			captchaCronJob.Run(cronJobCtx)
		})

		if errCron != nil {
			log.Println(errCron)
		}
	}()

	//server
	serverAddress := globalConfig.ServerAddress
	err = ginRouter.Gin.Run(serverAddress)
	if err != nil {
		log.Println(err)
		log.Fatal("could not start APIs")
	}

	defer cronScheduler.Stop()
}

func initialTests() {
	if _, err := os.Stat(util.FontComicPath); err != nil {
		panic(err.Error())
	}
}
