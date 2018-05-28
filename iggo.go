package main

import (
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"html/template"
	"time"
	"regexp"
	"io/ioutil"
	"github.com/gobuffalo/packr"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/jacoduplessis/simplejson"
	"net/url"
	"encoding/json"
	"os"
)

var tb = packr.NewBox("./templates")

var tmpl = template.New("__").Funcs(template.FuncMap{
	"thumbmax": thumbmax,
	"linkify":  linkify,
})

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
	ID         int
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

func GetPost(shortcode string) (interface{}, error) {

	path := fmt.Sprintf("/p/%s/", shortcode)
	b, err := get(path)
	if err != nil {
		return nil, err
	}
	return GetPostFromMarkup(b)
}

func GetPostFromMarkup(markup []byte) (interface{}, error) {

	sd := sharedData(markup)

	container, err := simplejson.NewJson(sd)
	if err != nil {
		return nil, err
	}
	base := container.GetPath("entry_data", "PostPage").GetIndex(0).GetPath("graphql", "shortcode_media")

	timestamp := base.Get("taken_at_timestamp").GetInt64()

	data := &Post{
		Shortcode: base.Get("shortcode").GetString(),
		ID:        base.Get("id").GetString(),
		URL:       base.Get("display_url").GetString(),
		Text:      getText(base),
		Timestamp: timestamp,
		Time:      humanize.Time(time.Unix(timestamp, 0)),
		Likes:     base.Get("edge_media_preview_like").Get("count").GetInt(),
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

func GetUserFromMarkup(markup []byte) (interface{}, error) {

	sd := sharedData(markup)

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

func GetTagFromMarkup(markup []byte) (interface{}, error) {

	sd := sharedData(markup)

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

func GetUser(username string) (interface{}, error) {

	path := fmt.Sprintf("/%s/", username)
	b, err := get(path)
	if err != nil {
		return nil, err
	}
	return GetUserFromMarkup(b)
}

func GetTag(slug string) (interface{}, error) {

	path := fmt.Sprintf("/explore/tags/%s/", slug)
	b, err := get(path)
	if err != nil {
		return nil, err
	}
	return GetTagFromMarkup(b)
}

func thumbmax(p *Post, w int) string {
	r := ""
	for _, s := range p.Sizes {
		if s.Width <= w {
			r = s.URL
		} else {
			break
		}
	}
	return r
}

func linkify(s string) template.HTML {

	t := regexp.MustCompile(`(?i)#([\p{L}\w]+)`)
	s = t.ReplaceAllString(s, `<a href="/tag/$1">#$1</a>`)
	u := regexp.MustCompile(`(?i)@([\p{L}\w.]+)`)
	s = u.ReplaceAllString(s, `<a href="/user/$1">@$1</a>`)
	return template.HTML(s)
}

func setupTemplates() {

	for _, n := range tb.List() {
		template.Must(tmpl.New(n).Parse(tb.String(n)))
	}
}

func renderTemplate(w http.ResponseWriter, templateName string, data interface{}) {

	err := tmpl.ExecuteTemplate(w, templateName, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func sharedData(b []byte) ([]byte) {

	re := regexp.MustCompile(`window._sharedData\s?=\s?(.*);</script>`)
	matches := re.FindSubmatch(b)
	if len(matches) < 2 {
		return []byte{}
	}
	return matches[1]

}

func getSearchResult(q string) (*SearchResult, error) {
	sr := &SearchResult{}
	qs := &url.Values{}
	qs.Add("context", "blended")
	qs.Add("query", q)
	r, err := client.Get("https://www.instagram.com/web/search/topsearch/?" + qs.Encode())
	defer r.Body.Close()
	if err != nil {
		return sr, err
	}
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return sr, err
	}
	err = json.Unmarshal(b, sr)
	return sr, err
}

func get(path string) ([]byte, error) {
	r, err := client.Get("https://www.instagram.com" + path)
	defer r.Body.Close()
	if err != nil {
		return []byte{}, err
	}
	return ioutil.ReadAll(r.Body)

}

func IndexHandler(w http.ResponseWriter, r *http.Request) {

	q := r.FormValue("q")
	if q != "" {
		sr, _ := getSearchResult(q)
		sr.Query = q
		renderTemplate(w, "search.html", sr)
		return
	}
	renderTemplate(w, "index.html", nil)
}

func makeHandler(fetcher func(string) (interface{}, error), varname string, templateName string) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		vars := mux.Vars(r)
		vardata, ok := vars[varname]
		if !ok {
			http.Error(w, "Invalid URL", http.StatusNoContent)
			return
		}

		data, err := fetcher(vardata)

		if err != nil {
			http.Error(w, "Could not load data", http.StatusNoContent)
			return
		}

		renderTemplate(w, templateName, data)
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

func main() {
	setupTemplates()
	r := mux.NewRouter()
	r.HandleFunc("/", IndexHandler)
	r.HandleFunc("/user/{username}", makeHandler(GetUser, "username", "user.html"))
	r.HandleFunc("/post/{shortcode}", makeHandler(GetPost, "shortcode", "post.html"))
	r.HandleFunc("/tag/{slug}", makeHandler(GetTag, "slug", "tag.html"))
	log.Fatal(http.ListenAndServe(getListenAddr(), r))
}
