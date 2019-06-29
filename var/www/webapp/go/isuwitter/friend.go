package main

import (
	"strings"

	"go.uber.org/zap"
)

type Friend struct {
	ID      int64  `db:"id"`
	Me      string `db:"me"`
	Friends string `db:"friends"`
}

func loadFriends(name string) ([]string, error) {
	friend := new(Friend)
	stmt, err := db.Prepare("SELECT * FROM friends WHERE me = ?")
	if err != nil {
		logger.Error(`db.Prepare("SELECT * FROM friends WHERE me = ?")`, zap.Error(err))
		return nil, err
	}
	err = stmt.QueryRow(name).Scan(&friend.ID, &friend.Me, &friend.Friends)
	if err != nil {
		logger.Error("stmt.QueryRow(name)", zap.Error(err))
		return nil, err
	}
	return strings.Split(friend.Friends, ","), nil
}
