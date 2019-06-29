package main

import (
	"context"
	"runtime/trace"
	"strings"

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

	friend := new(Friend)
	stmt, err := db.PrepareContext(ctx, "SELECT * FROM friends WHERE me = ?")
	if err != nil {
		logger.Error(`db.Prepare("SELECT * FROM friends WHERE me = ?")`, zap.Error(err))
		return ctx, nil, err
	}
	err = stmt.QueryRowContext(ctx, name).Scan(&friend.ID, &friend.Me, &friend.Friends)
	if err != nil {
		logger.Error("stmt.QueryRow(name)", zap.Error(err))
		return ctx, nil, err
	}
	return ctx, strings.Split(friend.Friends, ","), nil
}
