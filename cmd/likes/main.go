package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/janeczku/go-spinner"
	"github.com/vardius/progress-go"
)

const (
	LikesURL         = "https://confluence.int.aurora.tech/rest/likes/1.0/content/%s/likes"
	JiraHost         = "https://confluence.int.aurora.tech/"
	CqlURL           = "rest/api/content/?spaceKey=~leonr@aurora.tech&type=blogpost"
	AuthHeaderFormat = "Bearer %s"
	TokenPath        = "~/.jira/blogtoken"
)

type blogEntry struct {
	Id        string
	Title     string
	Likes     int
	Published time.Time
	Links     map[string]string `json:"_links"`
}

type cqlResponse struct {
	Results []blogEntry
	Start   int
	Limit   int
	Size    int
	Links   map[string]string `json:"_links"`
}

type user struct {
	Name                string
	FullName            string
	URL                 string
	AvatarURL           string
	FollowdByRemoteUser string
}

type like struct {
	User user
}

type likesResponse struct {
	Likes        []like
	Content_Type string
	Content_id   int
}

type JiraClient struct {
	jiraToken string
	client    *http.Client
}

func main() {

	jiraClient, err := NewJiraClient()
	if err != nil {
		log.Fatal(err)
	}

	allResults := LoadAllBlogs(jiraClient)

	pb := progress.New(0, int64(len(allResults)))
	pb.Start()
	for i := range allResults {
		pb.Advance(1)
		GetBlogDetails(jiraClient, allResults, i)
	}
	pb.Stop()

	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Likes < allResults[j].Likes
	})

	for _, blog := range allResults {
		fmt.Printf("Title: %s Date:%s likes: %d permalink: %s\n", blog.Title, blog.Published.Format("2006-01-02"), blog.Likes, blog.Links["tinyui"])
	}
}

func GetBlogDetails(jiraClient JiraClient, allResults []blogEntry, i int) {
	url := fmt.Sprintf(LikesURL, allResults[i].Id)
	req, _ := jiraClient.NewRequest(http.MethodGet, url, nil)
	res, err := jiraClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	bodyBytes, err := io.ReadAll(res.Body)
	defer res.Body.Close()
	if err != nil {
		log.Fatal(err)
	}

	var likes = likesResponse{}
	json.Unmarshal(bodyBytes, &likes)
	link := allResults[i].Links["webui"]
	if strings.Contains(link, "display") {
		parts := strings.Split(link, "/")
		year, _ := strconv.ParseInt(parts[3], 10, 32)
		month, _ := strconv.ParseInt(parts[4], 10, 32)
		day, _ := strconv.ParseInt(parts[5], 10, 32)
		allResults[i].Published = time.Date(int(year), time.Month(month), int(day), 0, 0, 0, 0, time.UTC)
	}
	allResults[i].Likes = len(likes.Likes)
}

func LoadAllBlogs(blogReader JiraClient) []blogEntry {
	spin := spinner.StartNew("Loading blog list...")
	allResults := []blogEntry{}
	limit := 0
	size := 0
	base := JiraHost
	next := CqlURL
	for limit == size {
		url := base + next
		blogs := LoadBlogsPage(blogReader, url)
		allResults = append(allResults, blogs.Results...)
		base = blogs.Links["base"]
		next = blogs.Links["next"]
		limit = blogs.Limit
		size = blogs.Size
	}
	spin.Stop()
	return allResults
}

func LoadBlogsPage(blogReader JiraClient, url string) cqlResponse {
	req, _ := blogReader.NewRequest(http.MethodGet, url, nil)
	res, err := blogReader.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	bodyBytes, err := io.ReadAll(res.Body)
	defer res.Body.Close()
	if err != nil {
		log.Fatal(err)
	}

	var blogs = cqlResponse{}
	json.Unmarshal(bodyBytes, &blogs)
	return blogs
}

func NewJiraClient() (JiraClient, error) {
	filePath := TokenPath
	if strings.HasPrefix(filePath, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return JiraClient{}, err
		}
		filePath = strings.Replace(filePath, "~", homeDir, 1)
	}
	jiraToken, err := os.ReadFile(filePath)
	if err != nil {
		return JiraClient{}, err
	}

	client := &http.Client{}
	br := JiraClient{
		jiraToken: string(jiraToken)[:len(jiraToken)-1],
		client:    client,
	}

	return br, nil
}

func (b JiraClient) NewRequest(method string, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf(AuthHeaderFormat, b.jiraToken))
	return req, nil
}

func (b JiraClient) Do(request *http.Request) (*http.Response, error) {
	return b.client.Do(request)
}
