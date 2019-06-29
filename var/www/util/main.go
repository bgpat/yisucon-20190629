package main

import (
    "database/sql"
    "fmt"
    "html"
    "log"
    "os"
    "regexp"
    "strings"
    "time"

    _ "github.com/go-sql-driver/mysql"
    "github.com/davecgh/go-spew/spew"
)

type Tweet struct {
    ID        int
    UserID    int
    Text      string
    CreatedAt time.Time

    UserName string
    HTML     string
    Time     string
}

var (
    rex = regexp.MustCompile("#(\\S+)(\\s|$)")
)

func htmlify(tweet string) string {
    tweet = strings.Replace(tweet, "&", "&amp;", -1)
    tweet = strings.Replace(tweet, "<", "&lt;", -1)
    tweet = strings.Replace(tweet, ">", "&gt;", -1)
    tweet = strings.Replace(tweet, "'", "&apos;", -1)
    tweet = strings.Replace(tweet, "\"", "&quot;", -1)

    tweet = rex.ReplaceAllStringFunc(tweet, func(tag string) string {
        return fmt.Sprintf("<a class=\"hashtag\" href=\"/hashtag/%s\">#%s</a>", tag[1:len(tag)], html.EscapeString(tag[1:len(tag)]))
    })
    return tweet
}
func main() {
    host := os.Getenv("ISUWITTER_DB_HOST")
    if host == "" {
        host = "localhost"
    }
    port := os.Getenv("ISUWITTER_DB_PORT")
    if port == "" {
        port = "3306"
    }
    user := os.Getenv("ISUWITTER_DB_USER")
    if user == "" {
        user = "root"
    }
    password := os.Getenv("ISUWITTER_DB_PASSWORD")
    dbname := os.Getenv("ISUWITTER_DB_NAME")
    if dbname == "" {
        dbname = "isuwitter"
    }

    var err error
    db, err := sql.Open("mysql", fmt.Sprintf(
        "%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&loc=Local&parseTime=true",
        user, password, host, port, dbname,
    ))
    if err != nil {
        log.Fatalf("Failed to connect to DB: %s.", err.Error())
    }

    rows, err := db.Query(`SELECT * FROM tweets`)
    for rows.Next() {
        t := Tweet{}
        err := rows.Scan(&t.ID, &t.UserID, &t.Text, &t.CreatedAt)
        if err != nil {
            spew.Dump(t)
            return
        }
        _, err = db.Exec(`UPDATE tweets SET text=? WHERE id=?`, htmlify(t.Text), t.ID)
        if err != nil {
            spew.Dump(t)
            return
        }
    }
}
