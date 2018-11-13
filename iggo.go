package main

import (
	"encoding/json"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/gorilla/feeds"
	"github.com/jacoduplessis/simplejson"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

var templateFuncs = template.FuncMap{
	"sizemax": sizemax,
	"linkify": linkify,
}

var templateMap = map[string]*template.Template{}

var client = &http.Client{
	Timeout: time.Second * 10,
}

type Size struct {
	URL    string
	Width  int
	Height int
}

type User struct {
	ID         string
	Name       string
	Username   string
	Bio        string
	Followers  int
	Following  int
	ProfilePic string
	Posts      []*Post
}
type Post struct {
	ID        string
	Timestamp int64
	Time      string
	URL       string
	Width     int
	Height    int
	Shortcode string
	Likes     int
	Sizes     []Size
	Thumbnail string
	Text      string
	Owner     *PostOwner
	Likers    []*PostLiker
}

type PostLiker struct {
	ProfilePic string
	Username   string
}

type PostOwner struct {
	ID         string
	ProfilePic string
	Username   string
	Name       string
}

type SearchResult struct {
	Query string
	Users []struct {
		User struct {
			Username   string `json:"username"`
			Name       string `json:"full_name"`
			ProfilePic string `json:"profile_pic_url"`
			Followers  int    `json:"follower_count"`
			Byline     string `json:"byline"`
		}
	}
	Tags []struct {
		Tag struct {
			Name       string `json:"name"`
			MediaCount int    `json:"media_count"`
		} `json:"hashtag"`
	} `json:"hashtags"`
}

type Tag struct {
	Name  string
	Posts []*Post
}

func GetPost(r *http.Request) (*Post, error) {

	shortcode := strings.TrimRight(r.URL.Path[len("/post/"):], "/")
	if shortcode == "" {
		return nil, nil
	}
	resp, err := client.Get(fmt.Sprintf("https://www.instagram.com/p/%s/", shortcode))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return GetPostFromMarkup(resp.Body)
}

func GetPostFromMarkup(body io.Reader) (*Post, error) {

	sd := sharedData(body)

	container, err := simplejson.NewJson(sd)
	if err != nil {
		return nil, err
	}
	base := container.GetPath("entry_data", "PostPage").GetIndex(0).GetPath("graphql", "shortcode_media")

	timestamp := base.Get("taken_at_timestamp").GetInt64()
	likers := []*PostLiker{}

	for _, edge := range base.GetPath("edge_media_preview_like", "edges").GetArray() {
		n := edge.Get("node")
		likers = append(likers, &PostLiker{
			ProfilePic: n.Get("profile_pic_url").GetString(),
			Username:   n.Get("username").GetString(),
		})
	}

	data := &Post{
		Shortcode: base.Get("shortcode").GetString(),
		ID:        base.Get("id").GetString(),
		URL:       base.Get("display_url").GetString(),
		Text:      getText(base),
		Timestamp: timestamp,
		Time:      humanize.Time(time.Unix(timestamp, 0)),
		Likes:     base.Get("edge_media_preview_like").Get("count").GetInt(),
		Likers:    likers,
		Owner: &PostOwner{
			ID:         base.GetPath("owner", "id").GetString(),
			ProfilePic: base.GetPath("owner", "profile_pic_url").GetString(),
			Username:   base.GetPath("owner", "username").GetString(),
			Name:       base.GetPath("owner", "full_name").GetString(),
		},
	}

	return data, nil
}

func getText(j *simplejson.Json) string {
	return j.GetPath("edge_media_to_caption", "edges").GetIndex(0).GetPath("node", "text").GetString()
}

func getPosts(j *simplejson.Json) []*Post {

	var posts []*Post

	for _, edge := range j.Get("edges").GetArray() {
		n := edge.Get("node")
		var sizes []Size

		for _, s := range n.Get("thumbnail_resources").GetArray() {
			sizes = append(sizes, Size{
				URL:    s.Get("src").GetString(),
				Width:  s.Get("config_width").GetInt(),
				Height: s.Get("config_width").GetInt(),
			})
		}
		timestamp := n.Get("taken_at_timestamp").GetInt64()

		posts = append(posts, &Post{
			ID:        n.Get("id").GetString(),
			Shortcode: n.Get("shortcode").GetString(),
			URL:       n.Get("display_url").GetString(),
			Timestamp: timestamp,
			Time:      humanize.Time(time.Unix(timestamp, 0)),
			Likes:     n.GetPath("edge_liked_by", "count").GetInt(),
			Sizes:     sizes,
			Text:      getText(n),
			Height:    n.GetPath("dimensions", "height").GetInt(),
			Width:     n.GetPath("dimensions", "width").GetInt(),
			Thumbnail: n.Get("thumbnail_src").GetString(),
		})
	}

	return posts
}

func GetUserFromMarkup(body io.Reader) (*User, error) {

	sd := sharedData(body)

	container, err := simplejson.NewJson(sd)
	if err != nil {
		return nil, err
	}
	base := container.GetPath("entry_data", "ProfilePage").GetIndex(0).GetPath("graphql", "user")

	data := &User{
		ID:         base.Get("id").GetString(),
		Name:       base.Get("full_name").GetString(),
		Username:   base.Get("username").GetString(),
		Bio:        base.Get("biography").GetString(),
		Followers:  base.GetPath("edge_followed_by", "count").GetInt(),
		Following:  base.GetPath("edge_follow", "count").GetInt(),
		ProfilePic: base.Get("profile_pic_url_hd").GetString(),
		Posts:      getPosts(base.Get("edge_owner_to_timeline_media")),
	}

	return data, nil
}

func GetTagFromMarkup(body io.Reader) (*Tag, error) {

	sd := sharedData(body)

	container, err := simplejson.NewJson(sd)
	if err != nil {
		return nil, err
	}
	base := container.GetPath("entry_data", "TagPage").GetIndex(0).GetPath("graphql", "hashtag")

	data := &Tag{
		Name:  base.Get("name").GetString(),
		Posts: getPosts(base.Get("edge_hashtag_to_media")),
	}

	return data, nil
}

// GetUserFromUsername takes a username, makes a request
// and parses the response into a User struct, returning a pointer
func GetUserFromUsername(username string) (*User, error) {

	if username == "" {
		return nil, nil
	}

	resp, err := client.Get(fmt.Sprintf("https://www.instagram.com/%s/", username))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return GetUserFromMarkup(resp.Body)

}

func GetUser(r *http.Request) (*User, error) {

	username := strings.TrimRight(r.URL.Path[len("/user/"):], "/")
	return GetUserFromUsername(username)
}

func GetTag(r *http.Request) (*Tag, error) {

	slug := strings.TrimRight(r.URL.Path[len("/tag/"):], "/")
	if slug == "" {
		return nil, nil
	}

	resp, err := client.Get(fmt.Sprintf("https://www.instagram.com/explore/tags/%s/", slug))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return GetTagFromMarkup(resp.Body)
}

func sizemax(p *Post, w int) Size {
	ix := 0
	for i, s := range p.Sizes {
		if s.Width <= w {
			ix = i
		} else {
			break
		}
	}
	return p.Sizes[ix]
}

func linkify(s string) template.HTML {

	t := regexp.MustCompile(`(?i)#([\p{L}\w]+)`)
	s = t.ReplaceAllString(s, `<a href="/tag/$1">#$1</a>`)
	u := regexp.MustCompile(`(?i)@([\p{L}\w.]+)`)
	s = u.ReplaceAllString(s, `<a href="/user/$1">@$1</a>`)
	return template.HTML(s)
}

func setupTemplates() {
	base := template.Must(template.ParseFiles("templates/base.html")).Funcs(templateFuncs)
	if _, err := base.ParseFiles("templates/custom.html"); err != nil {
		base.New("custom.html").Parse("")
	}

	keys := []string{"index", "post", "search", "tag", "user"}
	for _, key := range keys {
		clone := template.Must(base.Clone())
		tmpl := template.Must(clone.ParseFiles("templates/" + key + ".html"))
		templateMap[key] = tmpl
	}

}

func renderTemplate(w http.ResponseWriter, key string, data interface{}) *appError {

	tmpl, ok := templateMap[key]
	if !ok {
		return &appError{"Template error", 500, fmt.Errorf(`template "%s" not found`, key)}
	}
	err := tmpl.ExecuteTemplate(w, "base.html", data)
	if err != nil {
		return &appError{"Template error", 500, err}
	}
	return nil

}

func sharedData(r io.Reader) []byte {

	re := regexp.MustCompile(`window._sharedData\s?=\s?(.*);</script>`)
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil
	}
	matches := re.FindSubmatch(b)
	if len(matches) < 2 {
		return nil
	}
	return matches[1]

}

func getSearchResult(q string) (*SearchResult, error) {
	sr := &SearchResult{}
	qs := &url.Values{}
	qs.Add("context", "blended")
	qs.Add("query", q)
	r, err := client.Get("https://www.instagram.com/web/search/topsearch/?" + qs.Encode())

	if err != nil {
		return sr, err
	}
	defer r.Body.Close()

	err = json.NewDecoder(r.Body).Decode(sr)
	return sr, err
}

func renderJSON(w http.ResponseWriter, data interface{}) *appError {
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		return &appError{"Could not write response", 500, err}
	}
	return nil
}

func makeFeedHandler() http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		username := strings.TrimRight(r.URL.Path[len("/feed/"):], "/")
		user, err := GetUserFromUsername(username)
		if err != nil {
			log.Printf("Error fetching user (%s) data for feed: %s", username, err)
			w.Write([]byte("Error"))
			return
		}

		now := time.Now()
		feed := &feeds.Feed{
			Title:       fmt.Sprintf("Instagram Posts by %s", username),
			Link:        &feeds.Link{Href: fmt.Sprintf("https://www.instagram.com/%s", username)},
			Description: fmt.Sprintf("Recent photos posted by %s on Instagram", username),
			Created:     now,
		}

		for _, post := range user.Posts {

			item := feeds.Item{
				Id:      post.Shortcode,
				Title:   post.Text,
				Link:    &feeds.Link{Href: fmt.Sprintf("https://www.instagram.com/p/%s", post.Shortcode)},
				Author:  &feeds.Author{Name: username},
				Created: time.Unix(post.Timestamp, 0),
				Content: sizemax(post, 800).URL,
			}

			feed.Add(&item)

		}

		err = feed.WriteRss(w)
		if err != nil {
			log.Printf("Error writing feed: %s", err)
		}

	})
}

func makeIndex() appHandler {
	return func(w http.ResponseWriter, r *http.Request) *appError {

		q := r.FormValue("q")
		if q != "" {
			sr, _ := getSearchResult(q)
			sr.Query = q
			if r.URL.Query().Get("format") == "json" {
				return renderJSON(w, &sr)
			}
			return renderTemplate(w, "search", sr)

		}
		return renderTemplate(w, "index", nil)
	}
}

type appError struct {
	Message string
	Code    int
	Error   error
}

type appHandler func(w http.ResponseWriter, r *http.Request) *appError

func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if apperr := fn(w, r); apperr != nil {
		http.Error(w, apperr.Message, apperr.Code)
		log.Println(apperr.Error.Error())
	}
}

func makeHandler(f func(*http.Request) (interface{}, error), templateKey string) appHandler {

	return func(w http.ResponseWriter, r *http.Request) *appError {

		data, err := f(r)

		if err != nil || data == nil {
			return &appError{"Could not load data", 404, err}
		}

		if r.URL.Query().Get("format") == "json" {
			return renderJSON(w, &data)
		}

		return renderTemplate(w, templateKey, data)
	}
}

func getListenAddr() string {
	if port := os.Getenv("PORT"); port != "" {
		return ":" + port
	}
	if addr := os.Getenv("LISTEN_ADDR"); addr != "" {
		return addr
	}
	return "127.0.0.1:8000"
}

func userFetcher(r *http.Request) (interface{}, error) {
	return GetUser(r)
}

func postFetcher(r *http.Request) (interface{}, error) {
	return GetPost(r)
}

func tagFetcher(r *http.Request) (interface{}, error) {
	return GetTag(r)
}

func main() {
	setupTemplates()
	http.Handle("/user/", makeHandler(userFetcher, "user"))
	http.Handle("/post/", makeHandler(postFetcher, "post"))
	http.Handle("/tag/", makeHandler(tagFetcher, "tag"))
	http.Handle("/static/", http.StripPrefix("/static", http.FileServer(http.Dir("./static"))))
	http.Handle("/feed/", makeFeedHandler())
	http.Handle("/", makeIndex())
	addr := getListenAddr()
	fmt.Println("Listening on ", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
