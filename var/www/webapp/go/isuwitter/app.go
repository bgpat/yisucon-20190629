package main

import (
	"bytes"
	"crypto/sha1"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/go-redis/redis"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/unrolled/render"
	"go.uber.org/zap"
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

type User struct {
	ID       int
	Name     string
	Salt     string
	Password string
}

const (
	sessionName     = "isuwitter_session"
	sessionSecret   = "isuwitter"
	perPage         = 50
	isutomoEndpoint = "http://localhost:8081"
)

var (
	re              *render.Render
	rex             = regexp.MustCompile("#(\\S+)(\\s|$)")
	rexTweetHashTag = regexp.MustCompile(">#(%s)</a>")
	store           *sessions.FilesystemStore
	db              *sql.DB
	errInvalidUser  = errors.New("Invalid User")
	redisClient     *redis.Client
	logger, _       = zap.NewDevelopment()
)

func getuserID(name string) int {
	row := db.QueryRow(`SELECT id FROM users WHERE name = ?`, name)
	user := User{}
	err := row.Scan(&user.ID)
	if err != nil {
		return 0
	}
	return user.ID
}

func getUserName(id int) string {
	row := db.QueryRow(`SELECT name FROM users WHERE id = ?`, id)
	user := User{}
	err := row.Scan(&user.Name)
	if err != nil {
		return ""
	}
	return user.Name
}

func redisTweetStore(userName string, text string) error {
	err := redisClient.LPush("tweet-"+userName, time.Now().Format("2006-01-02 15:04:05")+"\t"+text).Err()
	if err != nil {
		logger.Error(
			"redisTweetStore",
			zap.Error(err),
			zap.String("userName", userName),
			zap.String("text", text),
		)
	}
	return err
}

func getHomeCache(name string) (string, error) {
	return redisClient.Get("home-" + name).Result()
}

func updateHomeCache(name string, home string) error {
	return redisClient.Set("home-"+name, home, 0).Err()
}

func clearHomeCache(name string) error {
	return redisClient.Del("home-" + name).Err()
}

func redisHashTweetStore(hashTag, userName, text string) {
	redisClient.LPush("hashtag-"+hashTag, userName+"\t"+time.Now().Format("2006-01-02 15:04:05")+"\t"+text)
}

func htmlify(tweet, userName string) string {
	tweet = strings.Replace(tweet, "&", "&amp;", -1)
	tweet = strings.Replace(tweet, "<", "&lt;", -1)
	tweet = strings.Replace(tweet, ">", "&gt;", -1)
	tweet = strings.Replace(tweet, "'", "&apos;", -1)
	tweet = strings.Replace(tweet, "\"", "&quot;", -1)

	tags := []string{}
	tweet = rex.ReplaceAllStringFunc(tweet, func(tag string) string {
		tags = append(tags, tag)
		return fmt.Sprintf("<a class=\"hashtag\" href=\"/hashtag/%s\">#%s</a>", tag[1:len(tag)], html.EscapeString(tag[1:len(tag)]))
	})

	for _, tag := range (tags) {
		redisHashTweetStore(tag, userName, tweet)
	}

	return tweet
}

func initializeHandler(w http.ResponseWriter, r *http.Request) {
	_, err := db.Exec(`DELETE FROM tweets WHERE id > 100000`)
	if err != nil {
		badRequest(w)
		return
	}

	_, err = db.Exec(`DELETE FROM users WHERE id > 1000`)
	if err != nil {
		badRequest(w)
		return
	}

	resp, err := http.Get(fmt.Sprintf("%s/initialize", isutomoEndpoint))
	if err != nil {
		badRequest(w)
		return
	}
	defer resp.Body.Close()

	{
		if err := exec.Command("systemctl", "stop", "redis").Run(); err != nil {
			logger.Error("failed to stop redis", zap.Error(err))
		}

		for {
			res, err := redisClient.Ping().Result()
			if err != nil {
				logger.Info("redis.Ping()", zap.Error(err))
				break
			}
			logger.Info("redis.Ping()", zap.String("result", res))
		}

		init, err := os.Open("/var/lib/redis/init.rdb")
		if err != nil {
			logger.Error("failed to open init.rdb", zap.Error(err))
			return
		}
		defer init.Close()
		dump, err := os.OpenFile("/var/lib/redis/dump.rdb", os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			logger.Error("failed to open dump.rdb", zap.Error(err))
			return
		}
		defer dump.Close()
		if _, err := io.Copy(dump, init); err != nil {
			logger.Error("failed to copy redis db", zap.Error(err))
		}

		if err := exec.Command("systemctl", "start", "redis").Run(); err != nil {
			logger.Error("failed to stop redis", zap.Error(err))
		}
	}

	re.JSON(w, http.StatusOK, map[string]string{"result": "ok"})
}

func initializeRedisHandler(w http.ResponseWriter, r *http.Request) {
	if err := redisClient.FlushDB().Err(); err != nil {
		badRequest(w)
		logger.Error("redisClient.FlushDB()", zap.Error(err))
		return
	}

	{
		// create init.rdb
		rows, err := db.Query(`SELECT * FROM tweets ORDER BY created_at`)
		if err != nil {
			badRequest(w)
			logger.Error("db.Query(`SELECT * FROM tweets ORDER BY created_at DESC`)", zap.Error(err))
			return
		}
		for rows.Next() {
			t := Tweet{}
			err := rows.Scan(&t.ID, &t.UserID, &t.Text, &t.CreatedAt)
			if err != nil {
				badRequest(w)
				logger.Error("rows.Scan(&t.ID, &t.UserID, &t.Text, &t.CreatedAt)", zap.Error(err))
				return
			}
			if err := redisTweetStore(getUserName(t.UserID), t.Text); err != nil {
				return
			}
			hashTag := rexTweetHashTag.FindStringSubmatch(t.Text)[0]
			redisClient.LPush("hashtag-"+hashTag, getUserName(t.UserID)+"\t"+t.CreatedAt.Format("2006-01-02 15:04:05")+"\t"+t.Text)
		}
	}

	if err := redisClient.Save().Err(); err != nil {
		badRequest(w)
		logger.Error("redisClient.FlushDB()", zap.Error(err))
		return
	}

	{
		// cp dump.rdb init.rdb
		dump, err := os.Open("/var/lib/redis/dump.rdb")
		if err != nil {
			logger.Error("failed to open init.rdb", zap.Error(err))
			return
		}
		defer dump.Close()
		init, err := os.OpenFile("/var/lib/redis/init.rdb", os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			logger.Error("failed to open dump.rdb", zap.Error(err))
			return
		}
		defer init.Close()
		if _, err := io.Copy(init, dump); err != nil {
			logger.Error("failed to copy redis db", zap.Error(err))
		}
	}
}

func topHandler(w http.ResponseWriter, r *http.Request) {
	var name string
	session := getSession(w, r)
	userID, ok := session.Values["user_id"]
	if ok {
		name = getUserName(userID.(int))
	}
	until := r.URL.Query().Get("until")

	if cache, err := getHomeCache(name); err == nil {
		w.Write([]byte(cache))
		return
	} else {
		logger.Debug(
			"cache miss",
			zap.Error(err),
			zap.String("name", name),
		)
	}

	if name == "" {
		flush, _ := session.Values["flush"].(string)
		session := getSession(w, r)
		session.Options = &sessions.Options{MaxAge: -1}
		session.Save(r, w)

		re.HTML(w, http.StatusOK, "index", struct {
			Name  string
			Flush string
		}{
			name,
			flush,
		})
		return
	}

	var rows *sql.Rows
	var err error
	if until == "" {
		rows, err = db.Query(`SELECT * FROM tweets ORDER BY created_at DESC`)
	} else {
		rows, err = db.Query(`SELECT * FROM tweets WHERE created_at < ? ORDER BY created_at DESC`, until)
	}

	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		badRequest(w)
		return
	}
	defer rows.Close()

	result, err := loadFriends(name)
	if err != nil {
		badRequest(w)
		return
	}

	tweets := make([]*Tweet, 0)
	for rows.Next() {
		t := Tweet{}
		err := rows.Scan(&t.ID, &t.UserID, &t.HTML, &t.CreatedAt)
		if err != nil && err != sql.ErrNoRows {
			badRequest(w)
			return
		}
		t.Time = t.CreatedAt.Format("2006-01-02 15:04:05")

		t.UserName = getUserName(t.UserID)
		if t.UserName == "" {
			badRequest(w)
			return
		}

		for _, x := range result {
			if x == t.UserName {
				tweets = append(tweets, &t)
				break
			}
		}
		if len(tweets) == perPage {
			break
		}
	}

	add := r.URL.Query().Get("append")
	if add != "" {
		re.HTML(w, http.StatusOK, "_tweets", struct {
			Tweets []*Tweet
		}{
			tweets,
		})
		return
	}

	var buf bytes.Buffer
	re.HTML(&buf, http.StatusOK, "index", struct {
		Name   string
		Tweets []*Tweet
	}{
		name, tweets,
	})
	if err := updateHomeCache(name, buf.String()); err != nil {
		logger.Error(
			"updateHomeCache",
			zap.Error(err),
			zap.String("name", name),
		)
		badRequest(w)
		return
	}
	w.Write(buf.Bytes())
}

func tweetPostHandler(w http.ResponseWriter, r *http.Request) {
	var name string
	session := getSession(w, r)
	userID, ok := session.Values["user_id"]
	if !ok {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	userName := getUserName(userID.(int))
	if userName == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	text := r.FormValue("text")
	if text == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	text = htmlify(text, userName)

	_, err := db.Exec(`INSERT INTO tweets (user_id, text, created_at) VALUES (?, ?, NOW())`, userID, text)
	redisTweetStore(userName, text)
	if err != nil {
		badRequest(w)
		return
	}

	if err := clearHomeCache(name); err != nil {
		logger.Error(
			"clearHomeCache",
			zap.Error(err),
			zap.String("name", name),
		)
		badRequest(w)
		return
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	row := db.QueryRow(`SELECT * FROM users WHERE name = ?`, name)
	user := User{}
	err := row.Scan(&user.ID, &user.Name, &user.Salt, &user.Password)
	if err != nil && err != sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err == sql.ErrNoRows || user.Password != fmt.Sprintf("%x", sha1.Sum([]byte(user.Salt+r.FormValue("password")))) {
		session := getSession(w, r)
		session.Values["flush"] = "ログインエラー"
		session.Save(r, w)
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	session := getSession(w, r)
	session.Values["user_id"] = user.ID
	session.Save(r, w)
	http.Redirect(w, r, "/", http.StatusFound)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	session := getSession(w, r)
	session.Options = &sessions.Options{MaxAge: -1}
	session.Save(r, w)
	http.Redirect(w, r, "/", http.StatusFound)
}

func followHandler(w http.ResponseWriter, r *http.Request) {
	var userName string
	session := getSession(w, r)
	userID, ok := session.Values["user_id"]
	if ok {
		u := getUserName(userID.(int))
		if u == "" {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		userName = u
	} else {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	jsonStr := `{"user":"` + r.FormValue("user") + `"}`
	req, err := http.NewRequest(http.MethodPost, isutomoEndpoint+pathURIEscape("/"+userName), bytes.NewBuffer([]byte(jsonStr)))

	if err != nil {
		badRequest(w)
		return
	}

	resp, err := http.DefaultClient.Do(req)

	if err != nil || resp.StatusCode != 200 {
		badRequest(w)
		return
	}

	if err := clearHomeCache(userName); err != nil {
		logger.Error(
			"clearHomeCache",
			zap.Error(err),
			zap.String("name", userName),
		)
		badRequest(w)
		return
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

func unfollowHandler(w http.ResponseWriter, r *http.Request) {
	var userName string
	session := getSession(w, r)
	userID, ok := session.Values["user_id"]
	if ok {
		u := getUserName(userID.(int))
		if u == "" {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		userName = u
	} else {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	jsonStr := `{"user":"` + r.FormValue("user") + `"}`
	req, err := http.NewRequest(http.MethodDelete, isutomoEndpoint+pathURIEscape("/"+userName), bytes.NewBuffer([]byte(jsonStr)))

	if err != nil {
		badRequest(w)
		return
	}

	resp, err := http.DefaultClient.Do(req)

	if err != nil || resp.StatusCode != 200 {
		badRequest(w)
		return
	}

	if err := clearHomeCache(userName); err != nil {
		logger.Error(
			"clearHomeCache",
			zap.Error(err),
			zap.String("name", userName),
		)
		badRequest(w)
		return
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

func getSession(w http.ResponseWriter, r *http.Request) *sessions.Session {
	session, _ := store.Get(r, sessionName)

	return session
}

func pathURIEscape(s string) string {
	return (&url.URL{Path: s}).String()
}

func badRequest(w http.ResponseWriter) {
	code := http.StatusBadRequest
	http.Error(w, http.StatusText(code), code)
}

func userHandler(w http.ResponseWriter, r *http.Request) {
	var name string
	session := getSession(w, r)
	sessionUID, ok := session.Values["user_id"]
	if ok {
		name = getUserName(sessionUID.(int))
	} else {
		name = ""
	}

	user := mux.Vars(r)["user"]
	mypage := user == name

	userID := getuserID(user)
	if userID == 0 {
		http.NotFound(w, r)
		return
	}

	isFriend := false
	if name != "" {
		result, err := loadFriends(name)
		if err != nil {
			badRequest(w)
			return
		}

		for _, x := range result {
			if x == user {
				isFriend = true
				break
			}
		}
	}

	until := r.URL.Query().Get("until")
	var rows *sql.Rows
	var err error

	tweets := make([]*Tweet, 0)
	if until == "" {
		//		rows, err = db.Query(`SELECT * FROM tweets WHERE user_id = ? ORDER BY created_at DESC`, userID)
		//redisClient.LPush("tweet-"+getUserName(userID.(int)), time.Now().String()+"\t"+text)
		lRange, err := redisClient.LRange("tweet-"+user, 0, 50).Result()
		if err != nil {
			badRequest(w)
			return
		}
		for _, tweet := range lRange {
			splited := strings.SplitN(tweet, "\t", 2)
			t := Tweet{}
			t.Time = splited[0]
			t.HTML = splited[1]
			t.UserName = user
			tweets = append(tweets, &t)
		}
	} else {
		rows, err = db.Query(`SELECT * FROM tweets WHERE user_id = ? AND created_at < ? ORDER BY created_at DESC`, userID, until)
		if err != nil {
			if err == sql.ErrNoRows {
				http.NotFound(w, r)
				return
			}
			badRequest(w)
			return
		}
		defer rows.Close()

		for rows.Next() {
			t := Tweet{}
			err := rows.Scan(&t.ID, &t.UserID, &t.HTML, &t.CreatedAt)
			if err != nil && err != sql.ErrNoRows {
				badRequest(w)
				return
			}
			t.Time = t.CreatedAt.Format("2006-01-02 15:04:05")
			t.UserName = user
			tweets = append(tweets, &t)
			if len(tweets) == perPage {
				break
			}
		}
	}

	add := r.URL.Query().Get("append")
	if add != "" {
		re.HTML(w, http.StatusOK, "_tweets", struct {
			Tweets []*Tweet
		}{
			tweets,
		})
		return
	}

	re.HTML(w, http.StatusOK, "user", struct {
		Name     string
		User     string
		Tweets   []*Tweet
		IsFriend bool
		Mypage   bool
	}{
		name, user, tweets, isFriend, mypage,
	})
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	var name string
	session := getSession(w, r)
	userID, ok := session.Values["user_id"]
	if ok {
		name = getUserName(userID.(int))
	} else {
		name = ""
	}

	until := r.URL.Query().Get("until")

	query := r.URL.Query().Get("q")
	tweets := make([]*Tweet, 0)

	if mux.Vars(r)["tag"] != "" && until == "" {
		//redisClient.LPush("hashtag-"+hashTag, userName+"\t"+time.Now().Format("2006-01-02 15:04:05")+"\t"+text)
		query = "#" + mux.Vars(r)["tag"]
		tag := mux.Vars(r)["tag"]
		lRange, err := redisClient.LRange("hashtag-"+tag, 0, 50).Result()
		if err != nil {
			badRequest(w)
			return
		}
		for _, tweet := range lRange {
			splited := strings.SplitN(tweet, "\t", 3)
			t := Tweet{}
			t.UserName = splited[0]
			t.Time = splited[1]
			t.HTML = splited[2]
			tweets = append(tweets, &t)
		}
	} else {

		var rows *sql.Rows
		var err error
		if until == "" {
			rows, err = db.Query(`SELECT * FROM tweets ORDER BY created_at DESC`)
		} else {
			rows, err = db.Query(`SELECT * FROM tweets WHERE created_at < ? ORDER BY created_at DESC`, until)
		}
		if err != nil {
			if err == sql.ErrNoRows {
				http.NotFound(w, r)
				return
			}
			badRequest(w)
			return
		}
		defer rows.Close()

		for rows.Next() {
			t := Tweet{}
			err := rows.Scan(&t.ID, &t.UserID, &t.HTML, &t.CreatedAt)
			if err != nil && err != sql.ErrNoRows {
				badRequest(w)
				return
			}
			t.Time = t.CreatedAt.Format("2006-01-02 15:04:05")
			t.UserName = getUserName(t.UserID)
			if t.UserName == "" {
				badRequest(w)
				return
			}
			if strings.Index(t.HTML, query) != -1 {
				tweets = append(tweets, &t)
			}

			if len(tweets) == perPage {
				break
			}
		}
	}

	add := r.URL.Query().Get("append")
	if add != "" {
		re.HTML(w, http.StatusOK, "_tweets", struct {
			Tweets []*Tweet
		}{
			tweets,
		})
		return
	}

	re.HTML(w, http.StatusOK, "search", struct {
		Name   string
		Tweets []*Tweet
		Query  string
	}{
		name, tweets, query,
	})
}

func js(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Write(fileRead("./public/js/script.js"))
}

func css(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css")
	w.Write(fileRead("./public/css/style.css"))
}

func fileRead(fp string) []byte {
	fs, err := os.Open(fp)

	if err != nil {
		return nil
	}

	defer fs.Close()

	l, err := fs.Stat()

	if err != nil {
		return nil
	}

	buf := make([]byte, l.Size())

	_, err = fs.Read(buf)

	if err != nil {
		return nil
	}

	return buf
}

func main() {
	redisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

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
	db, err = sql.Open("mysql", fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&loc=Local&parseTime=true",
		user, password, host, port, dbname,
	))
	if err != nil {
		log.Fatalf("Failed to connect to DB: %s.", err.Error())
	}

	store = sessions.NewFilesystemStore("", []byte(sessionSecret))

	re = render.New(render.Options{
		Directory: "views",
		Funcs: []template.FuncMap{
			{
				"raw": func(text string) template.HTML {
					return template.HTML(text)
				},
				"add": func(a, b int) int { return a + b },
			},
		},
	})

	r := mux.NewRouter()
	r.HandleFunc("/initialize", initializeHandler).Methods("GET")
	r.HandleFunc("/initialize_redis", initializeRedisHandler).Methods("GET")

	l := r.PathPrefix("/login").Subrouter()
	l.Methods("POST").HandlerFunc(loginHandler)
	r.HandleFunc("/logout", logoutHandler)

	r.PathPrefix("/css/style.css").HandlerFunc(css)
	r.PathPrefix("/js/script.js").HandlerFunc(js)

	s := r.PathPrefix("/search").Subrouter()
	s.Methods("GET").HandlerFunc(searchHandler)
	t := r.PathPrefix("/hashtag/{tag}").Subrouter()
	t.Methods("GET").HandlerFunc(searchHandler)

	n := r.PathPrefix("/unfollow").Subrouter()
	n.Methods("POST").HandlerFunc(unfollowHandler)
	f := r.PathPrefix("/follow").Subrouter()
	f.Methods("POST").HandlerFunc(followHandler)

	u := r.PathPrefix("/{user}").Subrouter()
	u.Methods("GET").HandlerFunc(userHandler)

	i := r.PathPrefix("/").Subrouter()
	i.Methods("GET").HandlerFunc(topHandler)
	i.Methods("POST").HandlerFunc(tweetPostHandler)

	log.Fatal(http.ListenAndServe(":8080", r))
}
