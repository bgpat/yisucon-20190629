package main

import "strings"

type Friend struct {
	ID      int64  `db:"id"`
	Me      string `db:"me"`
	Friends string `db:"friends"`
}

func loadFriends(name string) ([]string, error) {
	friend := new(Friend)
	stmt, err := db.Prepare("SELECT * FROM friends WHERE me = ?")
	if err != nil {
		return nil, err
	}
	err = stmt.QueryRow(name).Scan(&friend.ID, &friend.Me, &friend.Friends)
	if err != nil {
		return nil, err
	}
	return strings.Split(friend.Friends, ","), nil
}
