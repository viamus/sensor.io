package main

import (
	"log"
	"sync"
	"time"

	"github.com/go-redis/redis"
)

type redisSet struct {
	key            string
	value          string
	expirationTime time.Duration
}

type redisRepository struct {
	addr     string
	password string
	client   *redis.Client
	redisSet chan *redisSet
}

var redisRepositoryInstance *redisRepository
var redisRepositoryOnce sync.Once

func getRedisRepositoryInstance() *redisRepository {
	redisRepositoryOnce.Do(func() {
		redisRepositoryInstance = &redisRepository{redisSet: make(chan *redisSet)}
	})

	return redisRepositoryInstance
}

func initRedisRepository(r *redisRepository) {

	log.Println("Initializing Redis Connection ...")

	r.client = redis.NewClient(&redis.Options{
		Addr:         r.addr,
		DialTimeout:  10 * time.Second,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		PoolSize:     10,
		PoolTimeout:  30 * time.Second,
		Password:     r.password,
	})

	err := checkConnection(r.client)

	if err != nil {
		panic(err)
	}
}

func checkConnection(c *redis.Client) error {
	_, err := c.Ping().Result()
	if err != nil {
		return err
	}
	return nil
}

func setKey(c *redis.Client, setter redisSet) error {
	err := c.Set(setter.key, setter.value, setter.expirationTime).Err()
	return err
}

func (r *redisRepository) redisSetRun() {

	log.Println("Starting Redis Routine ...")

	for {
		select {
		case setter := <-r.redisSet:
			log.Println("Sending Redis Value", setter)
			setKey(r.client, *setter)
			break
		}
	}
}
