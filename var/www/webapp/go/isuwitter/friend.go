package main

import (
	"context"
	"runtime/trace"

	"go.uber.org/zap"
)

type Friend struct {
	ID      int64  `db:"id"`
	Me      string `db:"me"`
	Friends string `db:"friends"`
}

func loadFriends(pctx context.Context, name string) (context.Context, []string, error) {
	ctx, task := trace.NewTask(pctx, "loadFriends")
	defer task.End()

	friends, err := redisClient.SMembers("friends-" + name).Result()
	if err != nil {
		logger.Error("redis.SMembers", zap.Error(err))
		return ctx, nil, err
	}
	if len(friends) == 0 {
		logger.Warn("no friends", zap.String("name", name), zap.String("key", "friends-"+name))
	} else {
		logger.Debug("friends", zap.String("name", name), zap.String("key", "friends-"+name), zap.Int("len", len(friends)))
	}
	return ctx, friends, nil
}

func addFriend(me, friend string) error {
	err := redisClient.SAdd("friends-"+me, friend).Err()
	if err != nil {
		logger.Error("redis.SAdd", zap.Error(err))
	}
	return err
}

func removeFriend(me, friend string) error {
	err := redisClient.SRem("friends-"+me, friend).Err()
	if err != nil {
		logger.Error("redis.SRem", zap.Error(err))
	}
	return err
}
